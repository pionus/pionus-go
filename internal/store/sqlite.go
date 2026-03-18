package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pionus/pionus-go/internal/model"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dbPath string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS articles (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		slug       TEXT NOT NULL UNIQUE,
		title      TEXT NOT NULL,
		content    TEXT NOT NULL,
		author     TEXT NOT NULL DEFAULT 'Secbone',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_articles_created_at ON articles(created_at DESC);`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) GetArticle(slug string) (*model.Article, error) {
	a := &model.Article{}
	err := s.db.QueryRow(
		`SELECT id, slug, title, content, author, created_at, updated_at FROM articles WHERE slug = ?`,
		slug,
	).Scan(&a.ID, &a.Slug, &a.Title, &a.Content, &a.Author, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get article %q: %w", slug, err)
	}
	return a, nil
}

func (s *SQLiteStore) GetAdjacentArticles(slug string) (prev *model.Article, next *model.Article, err error) {
	// Previous: newer article (created_at > current, ORDER BY ASC, LIMIT 1)
	prev = &model.Article{}
	err = s.db.QueryRow(
		`SELECT id, slug, title, author, created_at, updated_at FROM articles
		 WHERE created_at > (SELECT created_at FROM articles WHERE slug = ?)
		 ORDER BY created_at ASC LIMIT 1`, slug,
	).Scan(&prev.ID, &prev.Slug, &prev.Title, &prev.Author, &prev.CreatedAt, &prev.UpdatedAt)
	if err != nil {
		prev = nil
	}

	// Next: older article (created_at < current, ORDER BY DESC, LIMIT 1)
	next = &model.Article{}
	err = s.db.QueryRow(
		`SELECT id, slug, title, author, created_at, updated_at FROM articles
		 WHERE created_at < (SELECT created_at FROM articles WHERE slug = ?)
		 ORDER BY created_at DESC LIMIT 1`, slug,
	).Scan(&next.ID, &next.Slug, &next.Title, &next.Author, &next.CreatedAt, &next.UpdatedAt)
	if err != nil {
		next = nil
	}

	return prev, next, nil
}

func (s *SQLiteStore) ListArticles(page, limit int) ([]*model.Article, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count articles: %w", err)
	}

	offset := (page - 1) * limit
	rows, err := s.db.Query(
		`SELECT id, slug, title, content, author, created_at, updated_at
		 FROM articles ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()

	var articles []*model.Article
	for rows.Next() {
		a := &model.Article{}
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Content, &a.Author, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan article: %w", err)
		}
		articles = append(articles, a)
	}
	return articles, total, rows.Err()
}

func (s *SQLiteStore) CreateArticle(a *model.Article) error {
	now := time.Now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	res, err := s.db.Exec(
		`INSERT INTO articles (slug, title, content, author, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.Slug, a.Title, a.Content, a.Author, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create article: %w", err)
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) UpdateArticle(slug string, a *model.Article) error {
	a.UpdatedAt = time.Now()
	result, err := s.db.Exec(
		`UPDATE articles SET title = ?, content = ?, updated_at = ? WHERE slug = ?`,
		a.Title, a.Content, a.UpdatedAt, slug,
	)
	if err != nil {
		return fmt.Errorf("update article %q: %w", slug, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article %q not found", slug)
	}
	return nil
}

func (s *SQLiteStore) DeleteArticle(slug string) error {
	result, err := s.db.Exec(`DELETE FROM articles WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete article %q: %w", slug, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article %q not found", slug)
	}
	return nil
}
