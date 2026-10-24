# Pionus Go

A self-hosted blog engine in Go, built on [arry](../arry) and SQLite. It is
designed as a drop-in replacement for a Typecho blog: Typecho-compatible
content semantics, a built-in admin UI, and a path for importing existing
content and URLs.

## Features

- Posts and independent pages, drafts, private and scheduled publishing
- Categories, tags, threaded comments with moderation
- Server-rendered public pages (Go templates) and a JSON API
- Admin UI at `/admin`: dashboard, Markdown editor with preview, media
  library, taxonomies, comments and settings
- bcrypt login, server-side sessions, CSRF protection, upload MIME validation
- SQLite with WAL, embedded migrations, single-binary deployment

## Quick start

```bash
cp config.yaml config.yaml.local    # optional
make dev
```

Open <http://localhost:8087/admin/setup> and create the first account, or
bootstrap from the CLI:

```bash
go run . -create-admin admin -admin-password 'change-me-please'
```

Public site: <http://localhost:8087/>. API: `/api/posts` (writes need
`PIONUS_AUTH_TOKEN` or an admin session).

## Migrating from Typecho

Export the old database read-only, then import it:

```bash
# on the Typecho server
TYPECHO_ROOT=/www/blog php typecho-export.php > typecho.json
tar -C /www/blog/usr/uploads -czf uploads.tgz .

# on the new host
pionus -config config.yaml -import-typecho typecho.json -typecho-dry-run
pionus -config config.yaml -import-typecho typecho.json -typecho-uploads ./uploads
PIONUS_ADMIN_PASSWORD='new-password' pionus -config config.yaml -create-admin Secbone
```

See `tools/README.md` for what is converted and the remaining gaps.

## Configuration

See `config.yaml` for server, timezone, database, upload and session settings,
and `.env.example` for environment overrides.

## Development

```bash
make test
make vet
make fmt
make docker
```

The project and the arry framework are developed together via `go.work`; see
`go.work.example` and `CLAUDE.md`.

## Status

Implemented: content model, storage layer, public pages, JSON API, admin UI,
authentication, uploads, markdown import. Next: Typecho data import and legacy
URL redirects, public comments, Typecho-compatible permalinks, feeds and
sitemap.
