# External themes

Pionus ships with the embedded **Paper & Ink** theme. Set `theme` to a
directory to overlay a custom theme:

```yaml
theme: "/app/themes/my-theme"
```

or:

```bash
PIONUS_THEME=/app/themes/my-theme
```

An empty value uses the embedded theme.

## Directory layout

```text
my-theme/
  pages/
    layout.html
    home.html
    post.html
    archive.html
    ...
  assets/
    main.css
    app.js
```

Only files you provide are overridden. Missing templates and assets fall back
to the embedded theme, so a theme can start with a single changed template.

The template engine loads `pages/*.html` with Go `html/template`. Templates use
the same data as the embedded theme.

## Notes

- Templates are cached; restart the server after changing theme code.
- `/assets/*` serves the merged custom and embedded asset filesystems.
- Admin templates are separate and are not affected by the public theme.
- Theme-specific settings are not available yet; use plugin/config options for
  now.
- The theme directory must exist and be a directory, otherwise startup fails.
