# Pionus-Go Personal Blog Backend

## Project Overview

Personal blog backend built with custom web framework **arry**. Serves articles via RESTful JSON API and SSR pages, backed by SQLite.

## Tech Stack

- **Language**: Go 1.24+
- **Framework**: arry (custom, github.com/pionus/arry)
- **API**: RESTful JSON
- **Database**: SQLite (modernc.org/sqlite, pure Go)
- **Config**: YAML + environment variables

## Architecture

### Directory Structure

```
pionus-go/
├── main.go                      # Entry point
├── config.go                    # Config loader (YAML + env)
├── config.yaml                  # Configuration file
├── internal/
│   ├── model/
│   │   └── article.go           # Article model
│   ├── store/
│   │   ├── store.go             # Store interface
│   │   └── sqlite.go            # SQLite implementation
│   ├── handler/
│   │   ├── article.go           # REST API handlers
│   │   └── page.go              # SSR page handlers
│   └── migrate/
│       └── markdown.go          # Markdown → SQLite migration
├── migrations/
│   └── 001_create_articles.sql  # Schema reference
├── markdowns/                   # Legacy markdown files
├── theme/                       # Frontend theme (git submodule)
├── Makefile                     # Build system
├── Dockerfile                   # Container build
└── .env.example                 # Environment variable template
```

### Key Patterns

1. **SQLite Storage** — Single-file database, no external dependencies
2. **Store Interface** — Clean abstraction for CRUD operations
3. **Slug-Based URLs** — URL-friendly article identifiers
4. **Handler Closures** — Dependency injection via closure pattern

## Configuration

`config.yaml`:
```yaml
server:
  addr: ":8087"
theme: default
database:
  path: "./data/pionus.db"
tls:
  cert: ""
  key: ""
```

Environment variable overrides:
- `PIONUS_AUTH_TOKEN` — Auth token for protected endpoints
- `PIONUS_ADDR` — Server address
- `PIONUS_DB_PATH` — Database path

## Routes

### REST API
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/articles` | No | List articles (?page=&limit=) |
| GET | `/api/articles/:slug` | No | Get article |
| POST | `/api/articles` | Yes | Create article |
| PUT | `/api/articles/:slug` | Yes | Update article |
| DELETE | `/api/articles/:slug` | Yes | Delete article |

### SSR Pages
| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Homepage |
| GET | `/article` | Article list page |
| GET | `/article/:slug` | Article detail page |

### Static
- `/assets` → `./theme/{theme}/assets`
- `/node_modules` → `./theme/{theme}/node_modules`
- `/web_modules` → `./theme/{theme}/web_modules`

## Build & Run

```bash
make dev          # Run with go run
make build        # Compile binary
make start        # Build + start (background)
make stop         # Stop server
make restart      # Stop + start
make migrate      # Migrate markdowns to SQLite
```

### Docker
```bash
docker build -t pionus .
docker run -p 8087:8087 -e PIONUS_AUTH_TOKEN=secret pionus
```

## Models

### Article
```go
type Article struct {
    ID        int64     `json:"id"`
    Slug      string    `json:"slug"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    Author    string    `json:"author"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## Store Interface

```go
type Store interface {
    GetArticle(slug string) (*model.Article, error)
    ListArticles(page, limit int) ([]*model.Article, int, error)
    CreateArticle(a *model.Article) error
    UpdateArticle(slug string, a *model.Article) error
    DeleteArticle(slug string) error
    Close() error
}
```

## Development Guidelines

### Adding New Routes
1. Define handler in `internal/handler/`
2. Register in `main.go` router
3. Use closure pattern: `func MyHandler(db store.Store) arry.Handler`

### Theme Development
1. Theme is git submodule in `theme/`
2. Views in `theme/{theme}/pages/`
3. Assets in `theme/{theme}/assets/`
4. Update: `git submodule update --remote`

### Migration
Run `make migrate` to import legacy markdown files into SQLite. Idempotent — safe to run multiple times.

## Troubleshooting

### Database issues
- Ensure `data/` directory is writable
- Check `config.yaml` database path
- SQLite file: `data/pionus.db`

### Theme not loading
- Update submodule: `git submodule update --init --recursive`
- Check `config.yaml` theme name

### Arry framework
- Source: github.com/pionus/arry (fetched via `go mod download`)
