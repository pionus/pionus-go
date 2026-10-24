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
pionus \
  -config config.yaml \
  -import-typecho typecho.json \
  -typecho-uploads /path/to/old/usr/uploads \
  -typecho-dry-run
```

This prints counts, missing attachments and warnings without writing.

## 3. Import

```bash
pionus \
  -config config.yaml \
  -import-typecho typecho.json \
  -typecho-uploads /path/to/old/usr/uploads
```

The importer is idempotent: posts and comments are matched by their Typecho
ID (`source = typecho`), taxonomies by slug, and redirects are upserted.
Re-running updates posts instead of duplicating them.

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
- Old URLs are inserted into `redirects`:
  - `/index.php/archives/<cid>/` and `/archives/<cid>/`
  - `/index.php/<page-slug>.html` and `/<page-slug>.html`
  - `/index.php`, `/blog/` → `/article`
- Attachments are copied to `upload.dir` and recorded in `media` when the
  source file exists.

## Known gaps

- Category, tag, year/month and feed URLs are not redirected yet; those pages
  still need to be implemented in the new front-end.
- Typecho `post_draft` autosave revisions are skipped by default. Pass
  `-typecho-drafts` to import them as drafts.
- Password-protected posts are stored but password gating is not implemented.
