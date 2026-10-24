# Pionus-Go Personal Blog

## Project Overview

A self-hosted blog engine built with the custom **arry** web framework. It is
being developed as a Go replacement for a Typecho (PHP) blog: same content
semantics, old URLs kept alive with redirects, and no runtime dependencies
beyond a single binary and SQLite.

## Tech Stack

- **Go** 1.25
- **Framework**: arry (`github.com/pionus/arry`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Markdown**: `github.com/yuin/goldmark` (GFM + raw HTML)
- **Config**: YAML + environment variables

## Local Development With arry

The blog and the framework are developed together. `go.work` (git-ignored)
points at `../arry`, so local builds pick up framework changes immediately:

```bash
cd ../arry && git pull
go build ./...          # uses ../arry through go.work
```

`go.mod` pins a published arry pseudo-version, so `docker build` and CI do not
need the sibling checkout. When arry changes are ready to ship, push arry and
bump the pin:

```bash
go get github.com/pionus/arry@<commit>
```

Rule of thumb: **generic HTTP/framework behavior belongs in arry; blog domain
logic belongs here.** Framework changes need tests in `../arry`.

## Directory Structure

```
pionus-go/
├── main.go                       # Entry point, routing, middleware
├── config.go                     # Config loader (YAML + env)
├── config.yaml
├── internal/
│   ├── model/                    # Domain types and constants
│   │   ├── post.go               # Post, types/statuses, excerpt helpers
│   │   ├── user.go               # User, roles
│   │   ├── taxonomy.go           # Category, Tag, Slugify
│   │   ├── comment.go
│   │   ├── media.go              # Media, Session
│   │   └── redirect.go
│   ├── store/
│   │   ├── store.go              # Store interfaces + filters + errors
│   │   ├── sqlite.go             # Connection, WAL, migration runner
│   │   ├── migrations/           # Embedded SQL migrations
│   │   ├── post.go
│   │   ├── taxonomy.go
│   │   ├── comment.go
│   │   ├── user.go
│   │   ├── media.go
│   │   └── option.go             # Options + redirects
│   ├── handler/
│   │   ├── post.go               # JSON API
│   │   └── page.go               # SSR pages
│   └── migrate/
│       └── markdown.go           # Legacy markdown import
├── markdowns/                    # Legacy markdown files
├── theme/                        # Frontend theme (git submodule)
├── Dockerfile
└── Makefile
```

## Configuration

`config.yaml`:

```yaml
server:
  addr: ":8087"
theme: default
database:
  path: "./data/pionus.db"
tls:
  cert: ""     # when set, arry uses Let's Encrypt autocert for these domains
  key: ""
```

Environment overrides:

| Variable | Purpose |
|---|---|
| `PIONUS_AUTH_TOKEN` | Token for write API calls |
| `PIONUS_ADDR` | Listen address |
| `PIONUS_DB_PATH` | SQLite file path |

Site settings (title, author, posts per page, comment switches) live in the
`options` table, seeded by the first migration. This mirrors Typecho's
`typecho_options` and keeps content configuration out of the process config.

## Routes

### JSON API

Writes require `Authorization: <token>` or `Authorization: Bearer <token>`.
The aliases under `/api/articles` are kept for compatibility.

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/posts` | No | List published posts (`page`, `limit`, `q`, `category`, `tag`) |
| GET | `/api/posts/:slug` | No | Get a post; drafts need auth |
| POST | `/api/posts` | Yes | Create a post |
| PUT | `/api/posts/:slug` | Yes | Update a post |
| DELETE | `/api/posts/:slug` | Yes | Delete a post |

### SSR Pages

| Method | Path | Description |
|---|---|---|
| GET | `/` | Cover page with the latest 5 posts |
| GET | `/article?page=N` | Archive grouped by year |
| GET | `/article/:slug` | Post detail (drafts visible to auth only) |
| GET | `/health` | Health check |

### Static

- `/assets` → `./theme/{theme}/assets`
- `/node_modules` → `./theme/{theme}/node_modules`
- `/web_modules` → `./theme/{theme}/web_modules`

## Data Model

`internal/store/migrations/001_init.sql` is the source of truth.

| Table | Purpose |
|---|---|
| `posts` | Posts and pages; `type` = `post`/`page`, `status` = `publish`/`draft`/`hidden`/`waiting` |
| `users` | Accounts and roles (`owner`/`admin`/`editor`/`contributor`) |
| `categories`, `tags` | Taxonomies |
| `post_categories`, `post_tags` | Many-to-many relations (Typecho-compatible) |
| `comments` | Threaded comments with `pending`/`approved`/`spam` |
| `media` | Uploaded files |
| `sessions` | Server-side login sessions (SHA-256 token hash) |
| `options` | Site settings |
| `redirects` | Legacy URL → new URL mapping for SEO-preserving migration |
| `schema_migrations` | Applied migration versions |

Semantics:

- A published post always has `published_at`; future values mean scheduled.
- Public reads filter on `status = publish` and `published_at <= now`.
- `excerpt` is generated from Markdown when not provided.
- Post password is exposed as `password` on input only and never in JSON output.
- Author display name resolves as `users.screen_name` → `users.username` →
  `options.site.author`.

## Store Interfaces

`store.Store` composes focused interfaces: `PostStore`, `TaxonomyStore`,
`CommentStore`, `UserStore`, `MediaStore`, `OptionStore`, `RedirectStore`.
Handlers depend on `store.Store`. Errors are matched with `errors.Is` against
`store.ErrNotFound` and `store.ErrConflict`.

## Build & Run

```bash
make dev          # go run .
make build        # compile binary
make start        # build + start in background
make stop
make restart
make migrate      # import markdowns/ into SQLite (idempotent)
make test
make vet
make fmt
make docker
```

Docker:

```bash
docker build -t pionus .
docker run -p 8087:8087 -e PIONUS_AUTH_TOKEN=secret \
  -v "$PWD/data:/app/data" -v "$PWD/uploads:/app/uploads" pionus
```

The image runs as UID 10001 and uses `go.mod` (not `go.work`) to build.

## Development Guidelines

1. **New route**: handler in `internal/handler/`, register in `main.go`.
2. **New storage method**: add to the relevant interface in `store.go`, then
   implement in the matching file; always update tests.
3. **Schema change**: add `002_*.sql`; never edit an applied migration. The
   runner records versions in `schema_migrations`.
4. **Markdown**: render through the shared goldmark instance in `page.go`.
5. **Theme data keys**: `Articles`, `Groups`, `Article`, `RenderedContent`,
   `ReadingTime`, `Prev`, `Next`, `CurrentPath` are consumed by the submodule.
6. **Money path**: public listings must never expose drafts, hidden posts, or
   scheduled posts.

## Current Status / Next Steps

Done: schema, store layer, post API, SSR pages, markdown import, WAL, build.

Pending for Typecho parity:

1. Bootstrap command / first-run admin account (bcrypt + sessions + CSRF).
2. Admin UI: dashboard, post editor, media upload, taxonomy and comment
   moderation, settings.
3. Typecho importer (`typecho_contents`/`metas`/`relationships`/`comments`/
   `users`), `usr/uploads` migration and 301 redirect map.
4. Permalinks matching Typecho (`/archives/:slug/`, `/category/:slug/`,
   `/tag/:slug/`), feeds, sitemap, search.
5. Theme rewrite in Go templates.
