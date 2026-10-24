package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func (s *SQLiteStore) GetOption(name string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM options WHERE name = ?`, name).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get option %q: %w", name, err)
	}
	return value, nil
}

func (s *SQLiteStore) SetOption(name, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO options (name, value) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value`,
		name, value,
	)
	if err != nil {
		return fmt.Errorf("set option %q: %w", name, err)
	}
	return nil
}

func (s *SQLiteStore) AllOptions() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT name, value FROM options`)
	if err != nil {
		return nil, fmt.Errorf("list options: %w", err)
	}
	defer rows.Close()

	options := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan option: %w", err)
		}
		options[name] = value
	}
	return options, rows.Err()
}

func (s *SQLiteStore) SetRedirect(from, to string, status int) error {
	if status == 0 {
		status = 301
	}
	_, err := s.db.Exec(
		`INSERT INTO redirects (from_path, to_path, status, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(from_path) DO UPDATE SET to_path = excluded.to_path, status = excluded.status`,
		from, to, status, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("set redirect %q: %w", from, err)
	}
	return nil
}

func (s *SQLiteStore) FindRedirect(from string) (*model.Redirect, error) {
	r := &model.Redirect{}
	err := s.db.QueryRow(
		`SELECT from_path, to_path, status, created_at FROM redirects WHERE from_path = ?`, from,
	).Scan(&r.From, &r.To, &r.Status, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find redirect: %w", err)
	}
	return r, nil
}

func (s *SQLiteStore) ListRedirects() ([]*model.Redirect, error) {
	rows, err := s.db.Query(
		`SELECT from_path, to_path, status, created_at FROM redirects ORDER BY from_path ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list redirects: %w", err)
	}
	defer rows.Close()

	redirects := make([]*model.Redirect, 0)
	for rows.Next() {
		r := &model.Redirect{}
		if err := rows.Scan(&r.From, &r.To, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan redirect: %w", err)
		}
		redirects = append(redirects, r)
	}
	return redirects, rows.Err()
}
