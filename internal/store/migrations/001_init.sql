-- Pionus schema. This is the baseline for fresh installs.
-- Existing databases keep their schema_migrations history; never edit an
-- applied migration in a deployed installation.

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL DEFAULT '',
    screen_name   TEXT NOT NULL DEFAULT '',
    url           TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'editor',
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL,
    last_login_at DATETIME
);

CREATE TABLE IF NOT EXISTS posts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    slug          TEXT NOT NULL UNIQUE,
    title         TEXT NOT NULL,
    content       TEXT NOT NULL DEFAULT '',
    excerpt       TEXT NOT NULL DEFAULT '',
    cover         TEXT NOT NULL DEFAULT '',
    type          TEXT NOT NULL DEFAULT 'post',
    status        TEXT NOT NULL DEFAULT 'draft',
    format        TEXT NOT NULL DEFAULT 'markdown',
    template      TEXT NOT NULL DEFAULT '',
    author_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    post_password TEXT NOT NULL DEFAULT '',
    allow_comment INTEGER NOT NULL DEFAULT 1,
    pinned        INTEGER NOT NULL DEFAULT 0,
    views         INTEGER NOT NULL DEFAULT 0,
    likes         INTEGER NOT NULL DEFAULT 0,
    published_at  DATETIME,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL,
    source        TEXT NOT NULL DEFAULT '',
    source_id     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_posts_public ON posts(type, status, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_posts_created ON posts(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts(author_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_source ON posts(source, source_id) WHERE source_id <> '';

CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    parent_id   INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS tags (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS post_categories (
    post_id     INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_post_categories_category ON post_categories(category_id);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_post_tags_tag ON post_tags(tag_id);

CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES comments(id) ON DELETE CASCADE,
    author     TEXT NOT NULL,
    email      TEXT NOT NULL DEFAULT '',
    url        TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    source     TEXT NOT NULL DEFAULT '',
    source_id  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comments_status ON comments(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comments_parent ON comments(parent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_comments_source ON comments(source, source_id) WHERE source_id <> '';

CREATE TABLE IF NOT EXISTS media (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT NOT NULL UNIQUE,
    filename   TEXT NOT NULL,
    mime       TEXT NOT NULL DEFAULT '',
    size       INTEGER NOT NULL DEFAULT 0,
    width      INTEGER NOT NULL DEFAULT 0,
    height     INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_media_created ON media(created_at DESC);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS options (
    name  TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    content    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'publish',
    source     TEXT NOT NULL DEFAULT '',
    source_id  TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_created ON notes(created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notes_source ON notes(source, source_id) WHERE source_id <> '';

CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
    title,
    content,
    post_id UNINDEXED,
    tokenize='trigram'
);

CREATE TRIGGER IF NOT EXISTS posts_fts_insert AFTER INSERT ON posts BEGIN
    INSERT INTO posts_fts(post_id, title, content) VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS posts_fts_update AFTER UPDATE OF title, content ON posts BEGIN
    DELETE FROM posts_fts WHERE post_id = old.id;
    INSERT INTO posts_fts(post_id, title, content) VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS posts_fts_delete AFTER DELETE ON posts BEGIN
    DELETE FROM posts_fts WHERE post_id = old.id;
END;

INSERT OR IGNORE INTO options (name, value) VALUES
    ('site.title', 'Secbone''s Blog'),
    ('site.description', ''),
    ('site.author', 'Secbone'),
    ('site.url', ''),
    ('site.locale', 'zh-CN'),
    ('posts.per_page', '20'),
    ('comments.enabled', '1'),
    ('comments.moderation', '1');
