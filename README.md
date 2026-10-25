# Pionus Go

A self-hosted blog engine in Go, built on [arry](../arry) and SQLite. It is a
Go replacement for a Typecho blog: compatible content semantics, a built-in
admin UI, and an importer for existing posts, pages and comments. Imported
content links are rewritten to new routes; old Typecho URLs can be 301
redirected with the optional `typecho-redirects` plugin.

## Features

- Posts and independent pages, drafts, private and scheduled publishing
- Categories, tags, threaded comments with moderation
- 微语 timeline for short posts, with a simple admin composer and permalinks
- Paper & Ink public theme (embedded, with external theme overlay): archive,
  category/tag/date pages, search, TOC, code copy, KaTeX display math plus
  opt-in inline `$...$`, dark mode, broken-image placeholders
- 传送门 friend-links page and 微语 timeline
- Native threaded comments with moderation, CSRF, honeypot, rate limiting and
  HTML sanitizing
- RSS feed, notes feed, comments feed, sitemap and robots.txt
- SQLite FTS5 trigram search, render caching and self-hosted code highlighting
- Password-protected posts and tags, signed draft preview links, admin password
  change and user roles
- Versioned multi-arch images published to GHCR with `latest` tracking the
  newest stable release
- Deployment manifests, backup script and CI
- Trusted-proxy aware client IPs for rate limiting
- Compile-time plugins with lifecycle hooks and a typed post/comment/note
  event model (`eventlog` reference plugin)
- Admin Typecho import wizard plus CLI/ZIP bundle import
- Optional `typecho-redirects` plugin for 301 legacy Typecho URLs
- External theme overlay via `theme` / `PIONUS_THEME`; missing templates fall
  back to the embedded theme
- Admin UI at `/admin`: dashboard, Markdown editor with preview, media
  library, taxonomies, comments and settings
- bcrypt login, server-side sessions, CSRF protection, upload MIME validation
- SQLite with WAL, embedded migrations and theme, single-binary deployment

## Quick start

```bash
cp config.yaml config.yaml.local    # optional
make dev
```

Or run the packaged image:

```bash
cp .env.example .env
docker compose up -d
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
TYPECHO_EXPORT=typecho.json TYPECHO_UPLOADS=./old-usr-uploads \
  ./scripts/migrate-typecho.sh --dry-run
TYPECHO_EXPORT=typecho.json TYPECHO_UPLOADS=./old-usr-uploads \
  ./scripts/migrate-typecho.sh
PIONUS_ADMIN_PASSWORD='new-password' ./pionus -config config.yaml -create-admin Secbone
```

The migration command runs import, link repair and image audit, and prints a
JSON report. It also imports the old `dreams` short-post table as the 微语
timeline and applies curated page overrides from `migration/pages` (传送门
friend links, 微语 pointer). Re-run with `--skip-import` to repair links on an
existing database.

Old Typecho URLs are intentionally not redirected. Imported content internal
links point at the new routes, so bookmarks to the old PHP addresses will 404.
See `tools/README.md` for what is converted and the remaining gaps. Use
`--force-pages` only when you want to re-apply `migration/pages` overrides on an
existing database. To audit old images:

```bash
pionus -config config.yaml -audit-images
```

## Configuration

See `config.yaml` for server, timezone, database, upload and session settings,
and `.env.example` for environment overrides.

## Deployment

See `deploy/README.md` for Kubernetes, Caddy and backup notes.

## Development

```bash
make test
make vet
make fmt
make docker
```

The project and the arry framework are developed together via `go.work`; see
`go.work.example`.

## Status

Implemented: content model, storage layer, public pages, Paper & Ink theme,
native comments, JSON API, admin UI, authentication, uploads, Typecho import,
RSS/sitemap, password-protected posts and tags, signed draft previews, user
management, FTS5 search, deployment/backup/CI assets and image audit.

Migration, hidden tags, external themes and plugin extension points are
documented in:

- [Install](docs/install.md)
- [Releases and images](docs/release.md)
- [Typecho migration](docs/typecho-migration.md)
- [Hidden tags](docs/tag-protection.md)
- [External themes](docs/themes.md)
- [Plugins and events](docs/plugins.md)
- [Design notes](docs/backlog.md)
