# Pionus-Go Personal Blog

## Project Overview

A self-hosted blog engine built with the custom **arry** web framework. It is
being developed as a Go replacement for a Typecho (PHP) blog: same content
semantics and no runtime dependencies
beyond a single binary and SQLite.

## Tech Stack

- **Go** 1.25
- **Framework**: arry (`github.com/pionus/arry`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Markdown**: `github.com/yuin/goldmark` (GFM + raw HTML)
- **Search**: SQLite FTS5 trigram
- **Code highlighting**: self-hosted highlight.js
- **Config**: YAML + environment variables

## Local Development With arry

The blog and the framework are developed together. A local `go.work`
(git-ignored, see `go.work.example`) points at `../arry`, so local builds pick
up framework changes immediately:

```bash
cp go.work.example go.work   # first checkout
cd ../arry && git pull
go build ./...               # uses ../arry through go.work
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
├── config.yaml
├── cmd/
│   └── pionus-migrate/           # Dedicated Typecho migration command
├── scripts/
│   ├── migrate-typecho.sh        # One-shot migration wrapper
│   └── backup.sh                 # SQLite + uploads backup
├── deploy/
│   ├── k8s/pionus.yaml           # Deployment, Service, PVC
│   └── README.md                 # Deployment/Caddy/backup notes
├── internal/
│   ├── config/                   # Config loader (YAML + env)
│   ├── server/                   # App wiring, middleware and routes
│   ├── model/                    # Domain types and constants
│   │   ├── post.go               # Post, types/statuses, excerpt helpers
│   │   ├── user.go               # User, roles
│   │   ├── taxonomy.go           # Category, Tag, Slugify
│   │   ├── comment.go
│   │   ├── media.go              # Media, Session
│   │   └── note.go               # 微语 short posts
│   ├── store/
│   │   ├── store.go              # Store interfaces + filters + errors
│   │   ├── sqlite.go             # Connection, WAL, migration runner
│   │   ├── migrations/           # Embedded SQL migrations
│   │   ├── post.go
│   │   ├── taxonomy.go
│   │   ├── comment.go
│   │   ├── user.go
│   │   ├── media.go
│   │   ├── note.go               # 微语 timeline
│   │   └── option.go             # Options
│   ├── content/
│   │   ├── post.go               # Shared post input validation
│   │   ├── markdown.go           # Shared goldmark renderer
│   │   └── render.go             # TOC, legacy HTML normalization, math flag
│   ├── auth/
│   │   └── auth.go               # bcrypt, sessions, RequireUser
│   ├── admin/
│   │   ├── admin.go              # Admin router, embedded templates, helpers
│   │   ├── auth_handlers.go      # Setup, login, logout
│   │   ├── posts.go              # Post list/editor/delete/preview
│   │   ├── taxonomy.go           # Categories and tags
│   │   ├── dashboard.go          # Dashboard and comment moderation
│   │   ├── notes.go              # Admin 微语 timeline
│   │   ├── media.go              # Uploads and media library
│   │   ├── settings.go           # Site settings
│   │   └── templates/            # Server-rendered admin UI (go:embed)
│   ├── imageaudit/
│   │   └── audit.go              # Image reference audit (local + online)
│   ├── handler/
│   │   ├── post.go               # JSON API
│   │   ├── page.go               # Home, archives, post detail
│   │   ├── comments.go           # Comment tree, submission, sanitizing
│   │   ├── notes.go              # 微语 timeline
│   │   ├── feed.go               # RSS, comments feed, sitemap, robots
│   │   ├── site.go               # Shared public page data
│   │   └── notfound.go           # Themed 404 catch-all
│   ├── migrate/
│   │   └── markdown.go           # Legacy markdown import
│   └── typecho/                  # Typecho JSON importer
│       ├── export.go
│       └── import.go
├── migration/
│   └── pages/                    # Curated overrides for old theme pages
│       ├── links.md              # 传送门 friend links
│       └── t.md                  # 微语 pointer
├── tools/
│   └── typecho-export.php        # Read-only Typecho exporter
├── web/
│   ├── embed.go                  # go:embed theme + template funcs
│   └── theme/
│       ├── pages/                # Paper & Ink public templates
│       └── assets/               # CSS, JS, KaTeX
├── markdowns/                    # Legacy markdown files
├── uploads/                      # Uploaded media (runtime)
├── theme/                        # Old Typecho theme submodule (unused)
├── Dockerfile
└── Makefile
```

## Configuration

`config.yaml`:

```yaml
server:
  addr: ":8087"
  access_log: "logs/access.log"  # "-" logs to stdout
timezone: "Asia/Shanghai"
security:
  secret: ""                     # set PIONUS_SECRET in production
database:
  path: "./data/pionus.db"
upload:
  dir: "./uploads"
  max_size_mb: 10
session:
  cookie_name: "pionus_session"
  ttl_hours: 168
  secure: false   # true when served over HTTPS
tls:
  cert: ""        # when set, arry uses Let's Encrypt autocert for these domains
  key: ""
```

Environment overrides:

| Variable | Purpose |
|---|---|
| `PIONUS_AUTH_TOKEN` | Token for write API calls |
| `PIONUS_ADDR` | Listen address |
| `PIONUS_DB_PATH` | SQLite file path |
| `PIONUS_TIMEZONE` | IANA timezone for timestamps |
| `PIONUS_SECRET` | HMAC secret for password-post cookies |
| `PIONUS_ACCESS_LOG` | Access log path; `-` means stdout |
| `PIONUS_TRUSTED_PROXIES` | Comma-separated proxy CIDRs/IPs for X-Forwarded-For |
| `PIONUS_UPLOAD_DIR` | Upload directory |
| `PIONUS_UPLOAD_MAX_MB` | Max upload size |
| `PIONUS_SESSION_COOKIE` | Session cookie name |
| `PIONUS_SESSION_TTL_HOURS` | Session lifetime |
| `PIONUS_SESSION_SECURE` | Mark cookies Secure (`1`/`true`) |
| `PIONUS_ADMIN_PASSWORD` | Password for `-create-admin` |

Site settings (title, author, posts per page, comment switches) live in the
`options` table, seeded by the first migration. This mirrors Typecho's
`typecho_options` and keeps content configuration out of the process config.

## Routes

### JSON API

Writes require `Authorization: <token>` or `Authorization: Bearer <token>`, or
an authenticated admin session. The aliases under `/api/articles` are kept for
compatibility.

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/posts` | No | List published posts (`page`, `limit`, `q`, `category`, `tag`) |
| GET | `/api/posts/:slug` | No | Get a post; drafts need auth |
| POST | `/api/posts` | Yes | Create a post |
| PUT | `/api/posts/:slug` | Yes | Update a post |
| DELETE | `/api/posts/:slug` | Yes | Delete a post |

### Admin UI

Mounted at `/admin`. Sessions use an HttpOnly cookie backed by the `sessions`
table; all POSTs require a CSRF token (double-submit cookie). The admin engine
serves templates embedded in the binary.

| Method | Path | Description |
|---|---|---|
| GET/POST | `/admin/setup` | First-run owner account (only while no users exist) |
| GET/POST | `/admin/login` | Sign in |
| POST | `/admin/logout` | Sign out |
| GET | `/admin` | Dashboard |
| GET | `/admin/posts` | Post list with filters |
| GET/POST | `/admin/posts/new` | Create a post |
| GET | `/admin/posts/:id/edit` | Edit a post |
| POST | `/admin/posts/:id` | Save a post |
| POST | `/admin/posts/:id/delete` | Delete a post |
| POST | `/admin/preview` | Render Markdown fragment for the editor preview |
| GET | `/admin/categories`, `/admin/tags` | Taxonomy management |
| GET/POST | `/admin/notes` | Create/delete/edit 微语 entries |
| GET/POST | `/admin/profile` | Change own password |
| GET/POST | `/admin/users` | Owner/admin user management |
| GET | `/admin/comments` | Comment moderation |
| GET | `/admin/media` | Media library and upload |
| GET/POST | `/admin/settings` | Site settings |

Bootstrap without the browser:

```bash
./pionus -create-admin admin -admin-password 'change-me-please'
```

### Public Pages

The Paper & Ink theme is embedded in the binary and served from `web/theme`.
Old Typecho URLs are intentionally not supported; they return the themed 404.

| Method | Path | Description |
|---|---|---|
| GET | `/` | Home: intro, latest posts, categories and tags |
| GET | `/article?page=N` | Archive grouped by year |
| GET | `/article/:slug` | Post/page detail with TOC, comments, math |
| POST | `/article/:slug/unlock` | Password-post access form |
| GET | `/article/:slug?preview=…` | Signed anonymous draft preview |
| POST | `/article/:slug/comment` | Native comment submission (CSRF, honeypot, time trap) |
| GET | `/category/:slug` | Category archive |
| GET | `/tag/:slug` | Tag archive |
| GET | `/archives/:year[/:month[/:day]]` | Date archives |
| GET | `/search?q=` | Search |
| GET | `/notes` | 微语 timeline |
| GET | `/notes/:id` | 微语 permalink |
| GET | `/categories`, `/tags` | Taxonomy indexes |
| GET | `/feed`, `/feed/notes`, `/feed/comments` | RSS 2.0 feeds |
| GET | `/sitemap.xml`, `/robots.txt` | SEO endpoints |
| GET | `/health` | Health check |

### Static

- `/assets` → embedded `web/theme/assets` (CSS, JS, KaTeX)
- `/uploads` → `upload.dir`

## Data Model

`internal/store/migrations/001_init.sql` is the source of truth.

| Table | Purpose |
|---|---|
| `posts` | Posts and pages; `type` = `post`/`page`, `status` = `publish`/`draft`/`hidden`/`waiting` |
| `users` | Accounts and roles (`owner`/`admin`/`editor`/`contributor`) |
| `categories`, `tags` | Taxonomies |
| `post_categories`, `post_tags` | Many-to-many relations (Typecho-compatible) |
| `comments` | Threaded comments with `pending`/`approved`/`spam` |
| `notes` | 微语 short posts for the timeline |
| `posts_fts` | Trigram FTS5 index maintained by triggers |
| `media` | Uploaded files |
| `sessions` | Server-side login sessions (SHA-256 token hash) |
| `options` | Site settings |
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
`CommentStore`, `UserStore`, `MediaStore`, `NoteStore`, `OptionStore`.
Handlers depend on `store.Store`. Errors are matched with `errors.Is` against
`store.ErrNotFound` and `store.ErrConflict`.

## Build & Run

```bash
make dev          # go run .
make build        # compile binary
make start        # build + start in background
make stop
make restart
make migrate          # import markdowns/ into SQLite (idempotent)
make migrate-typecho  # Typecho import + link repair + image audit
make migrate-links    # repair links / audit an existing database
go run . -audit-images  # report missing local images / external hosts
go run ./cmd/pionus-migrate -h  # dedicated migration command help
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

The image runs as UID 10001 and uses `go.mod` (not `go.work`) to build. It
contains both `pionus` and `pionus-migrate`. On first visit open `/admin/setup`,
or create the owner from the CLI before starting the server. In the container:

```bash
docker exec -it <container> ./pionus-migrate -config config.yaml \
  -export /tmp/typecho.json -source-uploads /tmp/usr/uploads
```

## Typecho Migration

`tools/typecho-export.php` is a read-only exporter. The dedicated migration
command performs import, link repair and image audit in one repeatable pass:

```bash
# old server (or a copy of the Typecho root)
TYPECHO_ROOT=/www/blog php typecho-export.php > typecho.json
tar -C /www/blog/usr/uploads -czf uploads.tgz .

# new host: dry run first
TYPECHO_EXPORT=typecho.json TYPECHO_UPLOADS=/path/to/usr/uploads \
  ./scripts/migrate-typecho.sh --dry-run
TYPECHO_EXPORT=typecho.json TYPECHO_UPLOADS=/path/to/usr/uploads \
  ./scripts/migrate-typecho.sh

# repair links / re-audit an existing database
./scripts/migrate-typecho.sh --skip-import

# imported users have no password
PIONUS_ADMIN_PASSWORD='new-password' ./pionus -config config.yaml -create-admin Secbone
```

The script wraps `go run ./cmd/pionus-migrate`, creates a timestamped SQLite
backup before writing, and emits a JSON report with `import`, `links`, `images`
and `verify` sections. Use `--skip-import` to repair links/audit an existing DB
and `--dry-run` to preview. The importer is idempotent (posts and
comments match on `source` + `source_id`), preserves HTML vs Markdown
(`posts.format`), imports categories, tags, relations, comments and views, copies
attachments when available, imports the old `dreams` short-post table as the
微语 timeline, applies curated page overrides from `migration/pages`, and runs
the whole import in one SQLite transaction. It
rewrites old internal post/page/category/tag, upload and feed links to the new
routes. Old Typecho URLs are intentionally not
redirected. Details and current gaps are in `tools/README.md`.

## Development Guidelines

1. **New route**: handler in `internal/handler/` (public) or
   `internal/admin/` (admin), register in `main.go` or `admin.Register`.
2. **Shared content rules** (slug/status validation, excerpt generation) belong
   in `internal/content`, not duplicated between API and admin.
3. **New storage method**: add to the relevant interface in `store.go`, then
   implement in the matching file; always update tests.
4. **Schema change**: add `002_*.sql`; never edit an applied migration. `001_init.sql`
   is the squashed baseline for fresh installs. The runner records versions in
   `schema_migrations`. `posts_fts` is maintained by triggers, so content updates
   must go through `posts` writes.
5. **Rendering**: use `content.RenderPostCached` for public post bodies. It
   handles Markdown/HTML, heading IDs, TOC, legacy HTML cleanup and math detection.
6. **Theme**: templates live in `web/theme/pages` and are embedded; data keys
   are built in `internal/handler` (`Posts`, `Groups`, `Post`, `Rendered`,
   `TOC`, `Comments`, `Form`, `CSRF`, ...).
7. **Money path**: public listings must never expose drafts or scheduled posts;
   admin POSTs and comment submissions must be CSRF-checked.
8. **Uploads**: validate by sniffed MIME, never by the client filename; keep
   the generated path relative and store it in `media.path`.

## Operations

- `scripts/backup.sh` creates a consistent SQLite backup and uploads archive.
- `deploy/k8s/pionus.yaml` runs one replica with a `Recreate` strategy because
  SQLite and uploads live on one PVC.
- CI (`.github/workflows/ci.yml`) runs gofmt, vet, tests and a Docker build.
- Production should set `PIONUS_SECRET`, `PIONUS_SESSION_SECURE=1`,
  `PIONUS_ACCESS_LOG=-` (stdout) and `PIONUS_TRUSTED_PROXIES` (the Caddy pod
  CIDR) behind Caddy.

## Current Status / Next Steps

Done: schema, store layer, JSON API, Paper & Ink public theme (home, archives,
post, TOC, code copy, KaTeX, responsive dark mode), native comments with
moderation and sanitizing, search, RSS/comments feeds, sitemap, image fallback,
embedded assets, and the Typecho importer with idempotent content, taxonomy,
comment and attachment handling, password-post gating, the 微语 timeline (from
the old `dreams` table), curated 传送门/微语 page overrides, FTS5 search, render
caching, code highlighting, user management and deployment/backup/CI assets.
Old Typecho URLs intentionally return 404.

Remaining polish:

1. Comment email notifications and a persistent/shared rate limiter.
2. Narrow handler dependencies and split large store interfaces.
3. Inline `$...$` math (currently `$$` / `\(` / `\[` only, to avoid PHP `$`).
4. Recover the three missing Typecho attachments if a backup exists.
5. Point DNS at the new host and run the production migration.
