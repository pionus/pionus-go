# Install Pionus

Pionus is a single Go binary plus a SQLite database and an uploads directory.

## Binary

```bash
go build -o pionus .
cp config.yaml config.local.yaml
PIONUS_SECRET='long-random-value' ./pionus -config config.local.yaml
```

Open `/admin/setup` to create the first owner account.

## Docker Compose

```bash
cp .env.example .env
# set PIONUS_SECRET, optionally PIONUS_AUTH_TOKEN
docker compose up -d
```

The compose file uses:

```text
./data       SQLite database
./uploads    uploaded files
./themes     optional external themes
./config.yaml
```

Enable built-in plugins in `.env`:

```bash
PIONUS_PLUGINS_ENABLED=typecho-redirects
```

Set an external theme only if the directory exists:

```bash
PIONUS_THEME=/app/themes/my-theme
```

## Reverse proxy

Pionus listens on `:8087` by default. Put Caddy/nginx in front for TLS and set:

```bash
PIONUS_SESSION_SECURE=1
PIONUS_TRUSTED_PROXIES=10.42.0.0/16
```

Set `site.url` in `/admin/settings` so canonical links, feeds and the sitemap
use the public URL.

## Backup

```bash
PIONUS_DB=./data/pionus.db PIONUS_UPLOADS=./uploads ./scripts/backup.sh
```

Back up both the SQLite database and uploads. SQLite WAL mode uses several
files; use the provided online backup script rather than copying only the
`.db` file while the service is running.
