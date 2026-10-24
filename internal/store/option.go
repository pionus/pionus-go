package store

import "fmt"

func (s *SQLiteStore) GetOption(name string) (string, error) {
	options, err := s.AllOptions()
	if err != nil {
		return "", err
	}
	value, ok := options[name]
	if !ok {
		return "", ErrNotFound
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
	s.optionsMu.Lock()
	s.optionsLoaded = false
	s.options = nil
	s.optionsMu.Unlock()
	return nil
}

func (s *SQLiteStore) AllOptions() (map[string]string, error) {
	s.optionsMu.RLock()
	if s.optionsLoaded {
		out := make(map[string]string, len(s.options))
		for k, v := range s.options {
			out[k] = v
		}
		s.optionsMu.RUnlock()
		return out, nil
	}
	s.optionsMu.RUnlock()

	s.optionsMu.Lock()
	defer s.optionsMu.Unlock()
	if s.optionsLoaded {
		out := make(map[string]string, len(s.options))
		for k, v := range s.options {
			out[k] = v
		}
		return out, nil
	}

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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.options = options
	s.optionsLoaded = true
	out := make(map[string]string, len(options))
	for k, v := range options {
		out[k] = v
	}
	return out, nil
}
