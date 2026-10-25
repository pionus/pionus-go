# Friend links

Friend links are managed in the admin instead of being edited as page HTML.

## Manage

Open:

```text
/admin/friends
```

You can add, edit, reorder, hide and delete links. Fields:

- name
- URL
- avatar URL (optional; the page falls back to the first character)
- description
- sort order
- visible

The public page is:

```text
/article/links
```

Its page body is only the intro text. The link grid is rendered from the
`friend_links` table, so content editors do not need to touch HTML.

## Initial data

Migration `004_friend_links.sql` seeds the current friend list. Existing
databases receive it automatically on startup.
