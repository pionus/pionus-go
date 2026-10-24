package store

import (
	"database/sql"
	"errors"
	"fmt"
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
