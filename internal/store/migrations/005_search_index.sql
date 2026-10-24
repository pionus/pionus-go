-- Trigram FTS index for Chinese substring search.
CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
    title,
    content,
    post_id UNINDEXED,
    tokenize='trigram'
);

INSERT INTO posts_fts(post_id, title, content)
SELECT id, title, content FROM posts
WHERE NOT EXISTS (
    SELECT 1 FROM posts_fts WHERE posts_fts.post_id = posts.id
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
