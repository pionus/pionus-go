-- Opt-in inline $...$ math. Historical PHP/shell posts keep this disabled
-- because dollar signs are common in code samples and prose.
ALTER TABLE posts ADD COLUMN math INTEGER NOT NULL DEFAULT 0;
