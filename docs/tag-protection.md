# Hidden tags

A tag can be marked **hidden** and given an access password. Posts carrying a
hidden tag are removed from public surfaces and require the tag password before
the article is shown.

This is useful for topics such as `网安` where posts should not appear in normal
navigation but remain reachable by readers who know the password.

## Configure

1. Open `/admin/tags`.
2. Check **隐藏标签** on the tag, for example `网安`.
3. Set **访问密码** and save.

The password is stored as a bcrypt hash. Leaving the password field empty while
editing keeps the existing password. A hidden tag cannot be created without a
password.

## Behavior

A post with any hidden tag is excluded from:

- home page
- article archive
- category, tag and date archives
- search
- RSS feeds and comments feed
- sitemap
- public JSON list API
- adjacent-post links

Its article URL still exists. An anonymous reader sees a password form instead
of the body. The tag archive URL (`/tag/<slug>`) is also password protected and
is not listed on `/tags`.

When a reader unlocks a tag, an HMAC-signed cookie is set for 24 hours. Changing
the tag password invalidates existing cookies. Admin sessions and signed draft
preview links bypass the gate.

Password pages are marked `noindex`, and the article body is not included in
metadata while locked.
