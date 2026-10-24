package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func (s *SQLiteStore) ListCategories() ([]*model.Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.description, c.parent_id, c.sort_order, c.created_at,
		       COUNT(DISTINCT pc.post_id) AS post_count
		FROM categories c
		LEFT JOIN post_categories pc ON pc.category_id = c.id
		GROUP BY c.id
		ORDER BY c.sort_order ASC, c.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := make([]*model.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (s *SQLiteStore) GetCategory(slug string) (*model.Category, error) {
	c, err := scanCategory(s.db.QueryRow(`
		SELECT c.id, c.name, c.slug, c.description, c.parent_id, c.sort_order, c.created_at,
		       COUNT(DISTINCT pc.post_id) AS post_count
		FROM categories c
		LEFT JOIN post_categories pc ON pc.category_id = c.id
		WHERE c.slug = ?
		GROUP BY c.id`, slug))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return c, nil
}

func scanCategory(sc scanner) (*model.Category, error) {
	c := &model.Category{}
	var parentID sql.NullInt64
	if err := sc.Scan(
		&c.ID, &c.Name, &c.Slug, &c.Description, &parentID, &c.SortOrder,
		&c.CreatedAt, &c.PostCount,
	); err != nil {
		return nil, err
	}
	if parentID.Valid {
		id := parentID.Int64
		c.ParentID = &id
	}
	return c, nil
}

func (s *SQLiteStore) CreateCategory(c *model.Category) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(
		`INSERT INTO categories (name, slug, description, parent_id, sort_order, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.Name, c.Slug, c.Description, c.ParentID, c.SortOrder, c.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: category slug %q already exists", ErrConflict, c.Slug)
		}
		return fmt.Errorf("create category: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) UpdateCategory(slug string, c *model.Category) error {
	res, err := s.db.Exec(
		`UPDATE categories SET name = ?, slug = ?, description = ?, parent_id = ?, sort_order = ?
		 WHERE slug = ?`,
		c.Name, c.Slug, c.Description, c.ParentID, c.SortOrder, slug,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: category slug %q already exists", ErrConflict, c.Slug)
		}
		return fmt.Errorf("update category %q: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteCategory(slug string) error {
	res, err := s.db.Exec(`DELETE FROM categories WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete category %q: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListTags() ([]*model.Tag, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.slug, t.created_at, COUNT(DISTINCT pt.post_id) AS post_count
		FROM tags t
		LEFT JOIN post_tags pt ON pt.tag_id = t.id
		GROUP BY t.id
		ORDER BY t.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*model.Tag, 0)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *SQLiteStore) GetTag(slug string) (*model.Tag, error) {
	t, err := scanTag(s.db.QueryRow(`
		SELECT t.id, t.name, t.slug, t.created_at, COUNT(DISTINCT pt.post_id) AS post_count
		FROM tags t
		LEFT JOIN post_tags pt ON pt.tag_id = t.id
		WHERE t.slug = ?
		GROUP BY t.id`, slug))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return t, nil
}

func scanTag(sc scanner) (*model.Tag, error) {
	t := &model.Tag{}
	if err := sc.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt, &t.PostCount); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *SQLiteStore) CreateTag(t *model.Tag) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(
		`INSERT INTO tags (name, slug, created_at) VALUES (?, ?, ?)`,
		t.Name, t.Slug, t.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: tag slug %q already exists", ErrConflict, t.Slug)
		}
		return fmt.Errorf("create tag: %w", err)
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) UpdateTag(slug string, t *model.Tag) error {
	res, err := s.db.Exec(
		`UPDATE tags SET name = ?, slug = ? WHERE slug = ?`, t.Name, t.Slug, slug,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: tag slug %q already exists", ErrConflict, t.Slug)
		}
		return fmt.Errorf("update tag %q: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteTag(slug string) error {
	res, err := s.db.Exec(`DELETE FROM tags WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete tag %q: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) UpsertTag(name, slug string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if slug == "" {
		slug = model.Slugify(name)
	}
	if slug == "" {
		return nil, fmt.Errorf("tag name or slug is required")
	}

	t, err := s.GetTag(slug)
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	t = &model.Tag{Name: name, Slug: slug}
	if err := s.CreateTag(t); err != nil {
		if errors.Is(err, ErrConflict) {
			// Another writer created it concurrently.
			return s.GetTag(slug)
		}
		return nil, err
	}
	return t, nil
}

func (s *SQLiteStore) SetPostCategories(postID int64, categoryIDs []int64) error {
	return replacePostCategories(s.db, postID, categoryIDs)
}

func (s *SQLiteStore) SetPostTags(postID int64, tagIDs []int64) error {
	return replacePostTags(s.db, postID, tagIDs)
}

func (s *SQLiteStore) postCategoriesBatch(ids []int64) (map[int64][]*model.Category, error) {
	out := make(map[int64][]*model.Category, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(`
		SELECT pc.post_id, c.id, c.name, c.slug, c.description, c.parent_id,
		       c.sort_order, c.created_at,
		       (SELECT COUNT(*) FROM post_categories pc2 WHERE pc2.category_id = c.id) AS post_count
		FROM post_categories pc
		JOIN categories c ON c.id = pc.category_id
		WHERE pc.post_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY pc.post_id, c.sort_order ASC, c.name ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("batch post categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID int64
		c := &model.Category{}
		var parentID sql.NullInt64
		if err := rows.Scan(&postID, &c.ID, &c.Name, &c.Slug, &c.Description, &parentID,
			&c.SortOrder, &c.CreatedAt, &c.PostCount); err != nil {
			return nil, fmt.Errorf("scan batch category: %w", err)
		}
		if parentID.Valid {
			v := parentID.Int64
			c.ParentID = &v
		}
		out[postID] = append(out[postID], c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) postTagsBatch(ids []int64) (map[int64][]*model.Tag, error) {
	out := make(map[int64][]*model.Tag, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.db.Query(`
		SELECT pt.post_id, t.id, t.name, t.slug, t.created_at,
		       (SELECT COUNT(*) FROM post_tags pt2 WHERE pt2.tag_id = t.id) AS post_count
		FROM post_tags pt
		JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY pt.post_id, t.name ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("batch post tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID int64
		t := &model.Tag{}
		if err := rows.Scan(&postID, &t.ID, &t.Name, &t.Slug, &t.CreatedAt, &t.PostCount); err != nil {
			return nil, fmt.Errorf("scan batch tag: %w", err)
		}
		out[postID] = append(out[postID], t)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) PostCategories(postID int64) ([]*model.Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.description, c.parent_id, c.sort_order, c.created_at,
		       COUNT(DISTINCT pc2.post_id) AS post_count
		FROM categories c
		JOIN post_categories pc ON pc.category_id = c.id
		LEFT JOIN post_categories pc2 ON pc2.category_id = c.id
		WHERE pc.post_id = ?
		GROUP BY c.id
		ORDER BY c.sort_order ASC, c.name ASC`, postID)
	if err != nil {
		return nil, fmt.Errorf("post categories: %w", err)
	}
	defer rows.Close()

	categories := make([]*model.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (s *SQLiteStore) PostTags(postID int64) ([]*model.Tag, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.slug, t.created_at, COUNT(DISTINCT pt2.post_id) AS post_count
		FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		LEFT JOIN post_tags pt2 ON pt2.tag_id = t.id
		WHERE pt.post_id = ?
		GROUP BY t.id
		ORDER BY t.name ASC`, postID)
	if err != nil {
		return nil, fmt.Errorf("post tags: %w", err)
	}
	defer rows.Close()

	tags := make([]*model.Tag, 0)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
