# Migrating from Typecho

This guide covers a self-hosted Typecho migration to Pionus, including an
optional bundle import and legacy URL redirects.

## 1. Back up Typecho

Back up both the database and uploads:

```bash
# database dump (example; adjust container/credentials)
mariadb-dump ... > typecho.sql

# old uploads
tar -C /path/to/typecho/usr/uploads -czf uploads.tgz .
```

Keep the old site and database available until the new deployment has been
verified.

## 2. Deploy Pionus somewhere else

Do not point the public domain at Pionus until the import has been checked.
Run Pionus with a fresh database and a persistent uploads directory.

Minimum configuration:

```yaml
timezone: "Asia/Shanghai"
database:
  path: "/app/data/pionus.db"
upload:
  dir: "/app/data/uploads"
security:
  secret: "long-random-value"
session:
  secure: true
```

Set `site.url` after the first import when the final domain is known.

## 3. Export Typecho

The exporter is a standalone PHP script:

```bash
TYPECHO_ROOT=/www/blog php tools/typecho-export.php > typecho.json
```

It reads the Typecho database and exports posts, pages, categories, tags,
comments, users, options and the custom `dreams` table.

## 4. Import

### Option A: JSON + uploads directory

```bash
pionus-migrate -config config.yaml \
  -export typecho.json \
  -source-uploads /path/to/usr/uploads \
  -pages migration/pages \
  -drafts
```

### Option B: ZIP bundle

Create a ZIP with `typecho.json` and the uploads directory:

```text
typecho-bundle.zip
  typecho.json
  uploads/
    2015/03/image.jpg
```

or:

```text
typecho-bundle.zip
  usr/uploads/
    2015/03/image.jpg
```

Then:

```bash
pionus-migrate -config config.yaml -bundle typecho-bundle.zip -drafts
```

The command creates a SQLite backup before writing by default. Use `-dry-run`
first.

### Option C: Admin import page

After logging in as an owner/admin, open:

```text
/admin/import
```

Upload the same ZIP bundle. The page supports dry-run, optional draft import,
and displays the JSON report. The upload limit is 512 MB; for very large sites
use the CLI.

## 5. Check the report

Important report fields:

- `posts`, `pages`, `comments`, `categories`, `tags`, `notes`
- `missing_uploads`
- `remaining_old_links`
- `skipped_drafts`
- `needs_password_reset`

Imported users have no usable password. Reset the owner account:

```bash
PIONUS_ADMIN_PASSWORD='new-password' \
  ./pionus -config config.yaml -create-admin <username>
```

## 6. Legacy Typecho URLs

Old Typecho URLs are optional. Enable the redirect plugin:

```yaml
plugins:
  enabled:
    - typecho-redirects
```

The plugin builds redirects from imported `source_id` values without a core
redirect table. It covers common permalinks:

```text
/archives/123/
/index.php/archives/123/
/?p=123
/index.php?p=123
/index.php/about.html
/index.php/category/tech/
/tag/go/
/usr/uploads/...
/search/keywords/
/feed/atom/
```

All are 301 redirects. Restart the service after importing new content so the
plugin rebuilds its in-memory map.

## 7. Verify and cut over

Before switching:

- home, archives, search, categories, tags
- article pages, images, code blocks, math
- comments and moderation
- password posts and hidden tags
- RSS, comments feed, sitemap, robots
- admin login and uploads
- old URL redirects

Then freeze old writing, run the import once more (it is idempotent), switch
Caddy/DNS to Pionus, and keep the old installation for rollback.

## Known gaps

- Typecho PHP themes and plugins do not run in Pionus.
- `typecho_fields` custom fields are exported but not mapped to a custom-field
  UI.
- Typecho plugin-specific data is not migrated automatically.
- Old URLs not covered by the redirect plugin can be added to the plugin.
- Missing upload files cannot be recovered from the database alone.
