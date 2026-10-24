package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func (s *SQLiteStore) CreateMedia(m *model.Media) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	res, err := s.db.Exec(`
		INSERT INTO media (path, filename, mime, size, width, height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Path, m.Filename, m.MIME, m.Size, m.Width, m.Height, m.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: media path %q already exists", ErrConflict, m.Path)
		}
		return fmt.Errorf("create media: %w", err)
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetMedia(id int64) (*model.Media, error) {
	m := &model.Media{}
	err := s.db.QueryRow(`
		SELECT id, path, filename, mime, size, width, height, created_at
		FROM media WHERE id = ?`, id,
	).Scan(&m.ID, &m.Path, &m.Filename, &m.MIME, &m.Size, &m.Width, &m.Height, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get media: %w", err)
	}
	return m, nil
}

func (s *SQLiteStore) ListMedia(page, limit int) ([]*model.Media, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count media: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT id, path, filename, mime, size, width, height, created_at
		FROM media ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		limit, (page-1)*limit,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()

	items := make([]*model.Media, 0, limit)
	for rows.Next() {
		m := &model.Media{}
		if err := rows.Scan(&m.ID, &m.Path, &m.Filename, &m.MIME, &m.Size, &m.Width, &m.Height, &m.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan media: %w", err)
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

func (s *SQLiteStore) DeleteMedia(id int64) error {
	res, err := s.db.Exec(`DELETE FROM media WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
