-- Short posts (微语) shown as a timeline.
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
