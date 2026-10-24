package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

// NoteFilter selects timeline notes.
type NoteFilter struct {
	Statuses []string
	Page     int
	Limit    int
}

func (s *SQLiteStore) ListNotes(f NoteFilter) ([]*model.Note, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	where := ""
	var args []any
	if len(f.Statuses) > 0 {
		placeholders := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		where = " WHERE status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notes`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notes: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT id, content, status, source, source_id, created_at, updated_at
		 FROM notes`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, f.Limit, (f.Page-1)*f.Limit)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	notes := make([]*model.Note, 0, f.Limit)
	for rows.Next() {
		n := &model.Note{}
		if err := rows.Scan(&n.ID, &n.Content, &n.Status, &n.Source, &n.SourceID, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, total, rows.Err()
}

func (s *SQLiteStore) GetNoteBySource(source, sourceID string) (*model.Note, error) {
	n := &model.Note{}
	err := s.db.QueryRow(
		`SELECT id, content, status, source, source_id, created_at, updated_at
		 FROM notes WHERE source = ? AND source_id = ?`, source, sourceID,
	).Scan(&n.ID, &n.Content, &n.Status, &n.Source, &n.SourceID, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get note by source: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) CreateNote(n *model.Note) error {
	now := time.Now()
	if n.Status == "" {
		n.Status = model.StatusPublish
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now

	res, err := s.db.Exec(
		`INSERT INTO notes (content, status, source, source_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		n.Content, n.Status, n.Source, n.SourceID, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: note already exists", ErrConflict)
		}
		return fmt.Errorf("create note: %w", err)
	}
	n.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) DeleteNote(id int64) error {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) CountNotes(status string) (int, error) {
	query := `SELECT COUNT(*) FROM notes`
	var args []any
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count notes: %w", err)
	}
	return n, nil
}
