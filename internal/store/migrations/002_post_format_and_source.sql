-- Preserve content format and Typecho origin for imports.

ALTER TABLE posts ADD COLUMN format TEXT NOT NULL DEFAULT 'markdown';
ALTER TABLE posts ADD COLUMN template TEXT NOT NULL DEFAULT '';
ALTER TABLE posts ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE posts ADD COLUMN source_id TEXT NOT NULL DEFAULT '';
ALTER TABLE posts ADD COLUMN likes INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_source ON posts(source, source_id) WHERE source_id <> '';

ALTER TABLE comments ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE comments ADD COLUMN source_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_comments_source ON comments(source, source_id) WHERE source_id <> '';
