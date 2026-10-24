-- Legacy Typecho URLs are intentionally not supported. Remove the unused
-- redirect table created by the initial import-oriented schema.
DROP TABLE IF EXISTS redirects;
