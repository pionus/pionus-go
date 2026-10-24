package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func (s *SQLiteStore) GetComment(id int64) (*model.Comment, error) {
	c := &model.Comment{}
	var parentID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT c.id, c.post_id, c.parent_id, c.author, c.email, c.url, c.content,
		       c.status, c.ip, c.user_agent, c.created_at
		FROM comments c WHERE c.id = ?`, id,
	).Scan(
		&c.ID, &c.PostID, &parentID, &c.Author, &c.Email, &c.URL, &c.Content,
		&c.Status, &c.IP, &c.UserAgent, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get comment: %w", err)
	}
	if parentID.Valid {
		v := parentID.Int64
		c.ParentID = &v
	}
	return c, nil
}

func (s *SQLiteStore) ListComments(f CommentFilter) ([]*model.Comment, int, error) {
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
	if f.PostID > 0 {
		where += " WHERE c.post_id = ?"
		args = append(args, f.PostID)
	}
	if f.Status != "" {
		if where == "" {
			where = " WHERE c.status = ?"
		} else {
			where += " AND c.status = ?"
		}
		args = append(args, f.Status)
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM comments c`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}

	rows, err := s.db.Query(`
		SELECT c.id, c.post_id, c.parent_id, c.author, c.email, c.url, c.content,
		       c.status, c.ip, c.user_agent, c.created_at,
		       p.title, p.slug
		FROM comments c
		LEFT JOIN posts p ON p.id = c.post_id`+where+`
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT ? OFFSET ?`, append(args, f.Limit, (f.Page-1)*f.Limit)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := make([]*model.Comment, 0, f.Limit)
	for rows.Next() {
		c := &model.Comment{}
		var parentID sql.NullInt64
		var postTitle, postSlug sql.NullString
		if err := rows.Scan(
			&c.ID, &c.PostID, &parentID, &c.Author, &c.Email, &c.URL, &c.Content,
			&c.Status, &c.IP, &c.UserAgent, &c.CreatedAt, &postTitle, &postSlug,
		); err != nil {
			return nil, 0, fmt.Errorf("scan comment: %w", err)
		}
		if parentID.Valid {
			v := parentID.Int64
			c.ParentID = &v
		}
		c.PostTitle = postTitle.String
		c.PostSlug = postSlug.String
		comments = append(comments, c)
	}
	return comments, total, rows.Err()
}

func (s *SQLiteStore) CreateComment(c *model.Comment) error {
	if c.Status == "" {
		c.Status = model.CommentPending
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}

	res, err := s.db.Exec(`
		INSERT INTO comments
			(post_id, parent_id, author, email, url, content, status, ip, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.PostID, c.ParentID, c.Author, c.Email, c.URL, c.Content,
		c.Status, c.IP, c.UserAgent, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) UpdateCommentStatus(id int64, status string) error {
	switch status {
	case model.CommentPending, model.CommentApproved, model.CommentSpam:
	default:
		return fmt.Errorf("invalid comment status %q", status)
	}

	res, err := s.db.Exec(`UPDATE comments SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update comment status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteComment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM comments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) CountComments(postID int64, status string) (int, error) {
	query := `SELECT COUNT(*) FROM comments WHERE post_id = ?`
	args := []any{postID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count comments: %w", err)
	}
	return n, nil
}
