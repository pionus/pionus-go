# Typecho migration

`typecho-export.php` is a read-only exporter for a Typecho 1.x database. It
prints normalized JSON and never writes to the database.

## 1. Export from the old server

Run it in the Typecho root (inside the PHP container if Typecho runs in one):

```bash
TYPECHO_ROOT=/www/blog php typecho-export.php > typecho.json
```

The exporter includes posts, pages, drafts (as `post_draft` rows), categories,
tags, relationships, comments, users (without password hashes) and site
options.

Copy the attachment directory as well if you want images:

```bash
tar -C /www/blog/usr/uploads -czf uploads.tgz .
```

## 2. Dry run on the new server

```bash
TYPECHO_EXPORT=typecho.json \
TYPECHO_UPLOADS=/path/to/old/usr/uploads \
  ./scripts/migrate-typecho.sh --dry-run
```

This imports nothing, repairs nothing and audits nothing; it prints the
counts and warnings that a real run would produce.

## 3. Import, repair links and audit

```bash
TYPECHO_EXPORT=typecho.json \
TYPECHO_UPLOADS=/path/to/old/usr/uploads \
  ./scripts/migrate-typecho.sh
```

The wrapper runs `go run ./cmd/pionus-migrate`, which:
1. backs up the SQLite database (unless `--dry-run`),
2. imports the JSON and attachments,
3. rewrites old internal links in posts and comments,
4. audits image references,
5. verifies counts and prints a JSON report.

Run just the repair/audit stages on an existing database:

```bash
./scripts/migrate-typecho.sh --skip-import
./scripts/migrate-typecho.sh --skip-import --dry-run
```

Useful flags: `--drafts`, `--online-audit`, `--skip-links`, `--skip-images`,
`--backup=false` and `--dry-run`. The JSON report contains `import`, `links`,
`images` and `verify` sections.

The importer is idempotent: posts and comments are matched by their Typecho
ID (`source = typecho`) and taxonomies by slug. Re-running updates posts
instead of duplicating them.

Typecho users are imported with no usable password. Reset the owner account:

```bash
PIONUS_ADMIN_PASSWORD='new-password' pionus -config config.yaml -create-admin Secbone
```

## What is converted

- `<!--markdown-->` posts become `format = markdown`; everything else is
  imported as raw HTML (`format = html`) and rendered without Markdown
  processing.
- Slugs keep Typecho's value when present; otherwise posts get
  `archives-<cid>` and pages get `page-<cid>`.
- `/usr/uploads/` links are rewritten to `/uploads/` in post and comment text.
- Internal links to old post/page/category/tag URLs are rewritten to the new
  routes (`/article/<slug>`, `/category/<slug>`, `/tag/<slug>`).
- Attachments are copied to `upload.dir` and recorded in `media` when the
  source file exists.

Old Typecho URLs are deliberately not redirected: `/index.php/...`,
`/<slug>.html`, `/2015/03/` and similar addresses return the new site's 404.
Bookmarks and external links to those addresses will break by design.

If the new site lives on a different domain, update `site.url` in the admin
settings after importing; feeds and the sitemap use it.

## Image audit

After importing, check image references:

```bash
pionus -config config.yaml -audit-images
pionus -config config.yaml -audit-images -audit-images-online
```

The first reports local `/uploads/` files that are missing and a host
histogram for external images. The second also checks external URLs (8
concurrent workers). The theme renders a placeholder for broken images and
keeps a link to the original.

## Known gaps

- Typecho `post_draft` autosave revisions are skipped by default. Pass
  `-drafts` to import them as drafts.
- Password-protected posts are stored but password gating is not implemented.
- The old server's `usr/uploads` directory was empty; three attachment files
  are missing unless another backup exists.
