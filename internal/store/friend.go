package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func (s *SQLiteStore) ListFriendLinks(includeHidden bool) ([]*model.FriendLink, error) {
	query := `SELECT id, name, url, avatar, description, sort_order, visible, created_at, updated_at
		FROM friend_links`
	if !includeHidden {
		query += ` WHERE visible = 1`
	}
	query += ` ORDER BY sort_order ASC, id ASC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list friend links: %w", err)
	}
	defer rows.Close()

	links := make([]*model.FriendLink, 0)
	for rows.Next() {
		link, err := scanFriendLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan friend link: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *SQLiteStore) GetFriendLink(id int64) (*model.FriendLink, error) {
	link, err := scanFriendLink(s.db.QueryRow(`
		SELECT id, name, url, avatar, description, sort_order, visible, created_at, updated_at
		FROM friend_links WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get friend link: %w", err)
	}
	return link, nil
}

func scanFriendLink(sc scanner) (*model.FriendLink, error) {
	link := &model.FriendLink{}
	var visible int
	if err := sc.Scan(
		&link.ID, &link.Name, &link.URL, &link.Avatar, &link.Description,
		&link.SortOrder, &visible, &link.CreatedAt, &link.UpdatedAt,
	); err != nil {
		return nil, err
	}
	link.Visible = visible != 0
	return link, nil
}

func (s *SQLiteStore) CreateFriendLink(link *model.FriendLink) error {
	now := time.Now()
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}
	link.UpdatedAt = now
	res, err := s.db.Exec(
		`INSERT INTO friend_links
			(name, url, avatar, description, sort_order, visible, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		link.Name, link.URL, link.Avatar, link.Description, link.SortOrder,
		boolInt(link.Visible), link.CreatedAt, link.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: friend link URL already exists", ErrConflict)
		}
		return fmt.Errorf("create friend link: %w", err)
	}
	link.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) UpdateFriendLink(link *model.FriendLink) error {
	link.UpdatedAt = time.Now()
	res, err := s.db.Exec(
		`UPDATE friend_links SET
			name = ?, url = ?, avatar = ?, description = ?, sort_order = ?, visible = ?,
			updated_at = ?
		 WHERE id = ?`,
		link.Name, link.URL, link.Avatar, link.Description, link.SortOrder,
		boolInt(link.Visible), link.UpdatedAt, link.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: friend link URL already exists", ErrConflict)
		}
		return fmt.Errorf("update friend link %d: %w", link.ID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteFriendLink(id int64) error {
	res, err := s.db.Exec(`DELETE FROM friend_links WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete friend link %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
