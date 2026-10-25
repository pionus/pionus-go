CREATE TABLE IF NOT EXISTS friend_links (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL UNIQUE,
    avatar      TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    visible     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_friend_links_order ON friend_links(visible, sort_order, id);

INSERT OR IGNORE INTO friend_links
    (name, url, avatar, description, sort_order, visible, created_at, updated_at)
VALUES
    ('Akawa', 'https://blog.akawa.ink/', 'https://avatars.githubusercontent.com/u/801928?v=4&s=96', 'blog.akawa.ink', 10, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Qiao Anran', 'https://github.com/qar', 'https://avatars.githubusercontent.com/u/2390326?v=4&s=96', 'github.com/qar', 20, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Yizero', 'https://github.com/yizero', 'https://avatars.githubusercontent.com/u/1787249?v=4&s=96', 'github.com/yizero', 30, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('GaleSun', 'https://galesun.com/', 'https://avatars.githubusercontent.com/u/10398241?v=4&s=96', 'galesun.com', 40, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('大象笔记', 'https://www.sunzhongwei.com/', 'https://avatars.githubusercontent.com/u/582045?v=4&s=96', 'sunzhongwei.com', 50, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('dilfish', 'https://dilfish.github.io/', 'https://avatars.githubusercontent.com/u/778853?v=4&s=96', 'dilfish.github.io', 60, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('捕蛇者说', 'http://www.bugcode.cn/', 'https://avatars.githubusercontent.com/u/1953662?v=4&s=96', 'bugcode.cn', 70, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('约瑟夫.岑', 'https://cinjoseph.github.io/', 'https://avatars.githubusercontent.com/u/9085831?v=4&s=96', 'cinjoseph.github.io', 80, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('Panda', 'https://panxinwu.github.io/', 'https://avatars.githubusercontent.com/u/1515508?v=4&s=96', 'panxinwu.github.io', 90, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('范明非', 'https://fanmingfei.com/', 'https://avatars.githubusercontent.com/u/4632277?v=4&s=96', 'fanmingfei.com', 100, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('skyjerry', 'https://github.com/skyjerry', 'https://avatars.githubusercontent.com/u/16223787?v=4&s=96', 'github.com/skyjerry', 110, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
