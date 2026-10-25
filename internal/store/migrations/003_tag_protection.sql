-- Hidden tags are omitted from public listings and their posts require the
-- tag password to read.
ALTER TABLE tags ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tags ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
