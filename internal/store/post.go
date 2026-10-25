package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pionus/pionus-go/internal/model"
)

const postColumnTemplate = `
	p.id, p.slug, p.title, %s, p.excerpt, p.cover, p.type, p.status,
	p.author_id, COALESCE(NULLIF(u.screen_name, ''), NULLIF(u.username, ''),
		(SELECT value FROM options WHERE name = 'site.author'), '') AS author,
	p.post_password, p.allow_comment, p.pinned, p.views, p.likes,
	p.format, p.template, p.math, p.source, p.source_id,
	p.published_at, p.created_at, p.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanPost(s scanner, withContent bool) (*model.Post, error) {
	p := &model.Post{}
	var (
		authorID     sql.NullInt64
		allowComment int
		pinned       int
		math         int
		publishedAt  sql.NullTime
	)
	if err := s.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt, &p.Cover,
		&p.Type, &p.Status, &authorID, &p.Author, &p.PostPassword,
		&allowComment, &pinned, &p.Views, &p.Likes,
		&p.Format, &p.Template, &math, &p.Source, &p.SourceID,
		&publishedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if authorID.Valid {
		id := authorID.Int64
		p.AuthorID = &id
	}
	p.AllowComment = allowComment != 0
	p.Pinned = pinned != 0
	p.Math = math != 0
	if publishedAt.Valid {
		t := publishedAt.Time
		p.PublishedAt = &t
	}

	if p.Format == "" {
		p.Format = model.FormatMarkdown
	}

	if !withContent {
		p.Content = ""
	} else if p.Excerpt == "" && p.Content != "" {
		p.Excerpt = model.MakeExcerpt(p.Content, 200)
	}

	return p, nil
}

func (s *SQLiteStore) GetPost(slug string) (*model.Post, error) {
	return s.getPost(slug, true)
}

func (s *SQLiteStore) GetPostByID(id int64) (*model.Post, error) {
	return s.getPostBy("p.id", id, true)
}

// GetPostBySource finds imported content by its origin (for example Typecho
// cid), which makes re-imports idempotent.
func (s *SQLiteStore) GetPostBySource(source, sourceID string) (*model.Post, error) {
	return s.getPostBy("(p.source || '/' || p.source_id)", source+"/"+sourceID, true)
}

func (s *SQLiteStore) getPost(slug string, withTaxonomy bool) (*model.Post, error) {
	return s.getPostBy("p.slug", slug, withTaxonomy)
}

func (s *SQLiteStore) getPostBy(column string, value any, withTaxonomy bool) (*model.Post, error) {
	query := fmt.Sprintf(
		`SELECT `+postColumnTemplate+`
		 FROM posts p LEFT JOIN users u ON u.id = p.author_id
		 WHERE %s = ?`, "p.content", column)

	p, err := scanPost(s.db.QueryRow(query, value), true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get post: %w", err)
	}

	if withTaxonomy {
		if p.Categories, err = s.PostCategories(p.ID); err != nil {
			return nil, err
		}
		if p.Tags, err = s.PostTags(p.ID); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func (s *SQLiteStore) ListPosts(f PostFilter) ([]*model.Post, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	where, args := postWhere(f)

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM posts p`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	contentExpr := "''"
	if f.WithContent {
		contentExpr = "p.content"
	}

	order := `ORDER BY p.pinned DESC, COALESCE(p.published_at, p.created_at) DESC, p.id DESC`
	if f.Query != "" {
		// Keep ordering deterministic for search results too.
		order = `ORDER BY p.pinned DESC, COALESCE(p.published_at, p.created_at) DESC, p.id DESC`
	}

	query := fmt.Sprintf(
		`SELECT `+postColumnTemplate+`
		 FROM posts p LEFT JOIN users u ON u.id = p.author_id`+where+`
		 %s LIMIT ? OFFSET ?`, contentExpr, order)

	rows, err := s.db.Query(query, append(args, f.Limit, (f.Page-1)*f.Limit)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	posts := make([]*model.Post, 0, f.Limit)
	for rows.Next() {
		p, err := scanPost(rows, f.WithContent)
		if err != nil {
			return nil, 0, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate posts: %w", err)
	}

	if f.WithTaxonomy {
		ids := make([]int64, 0, len(posts))
		for _, p := range posts {
			ids = append(ids, p.ID)
		}
		categories, err := s.postCategoriesBatch(ids)
		if err != nil {
			return nil, 0, err
		}
		tags, err := s.postTagsBatch(ids)
		if err != nil {
			return nil, 0, err
		}
		for _, p := range posts {
			p.Categories = categories[p.ID]
			p.Tags = tags[p.ID]
		}
	}

	return posts, total, nil
}

func postWhere(f PostFilter) (string, []any) {
	var clauses []string
	var args []any

	if f.Type != "" {
		clauses = append(clauses, "p.type = ?")
		args = append(args, f.Type)
	}

	if len(f.Statuses) > 0 {
		placeholders := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		clauses = append(clauses, "p.status IN ("+strings.Join(placeholders, ", ")+")")
	}

	if f.CategorySlug != "" {
		clauses = append(clauses, `EXISTS (
			SELECT 1 FROM post_categories pc
			JOIN categories c ON c.id = pc.category_id
			WHERE pc.post_id = p.id AND c.slug = ?)`)
		args = append(args, f.CategorySlug)
	}

	if f.TagSlug != "" {
		clauses = append(clauses, `EXISTS (
			SELECT 1 FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id
			WHERE pt.post_id = p.id AND t.slug = ?)`)
		args = append(args, f.TagSlug)
	}

	if f.Query != "" {
		if utf8.RuneCountInString(f.Query) >= 3 {
			clauses = append(clauses, "p.id IN (SELECT post_id FROM posts_fts WHERE posts_fts MATCH ?)")
			args = append(args, ftsPhrase(f.Query))
		} else {
			like := "%" + f.Query + "%"
			clauses = append(clauses, "(p.title LIKE ? OR p.content LIKE ?)")
			args = append(args, like, like)
		}
	}

	if f.PublishedBefore != nil {
		clauses = append(clauses, "p.published_at IS NOT NULL AND p.published_at <= ?")
		args = append(args, *f.PublishedBefore)
	}
	if f.PublishedFrom != nil {
		clauses = append(clauses, "p.published_at >= ?")
		args = append(args, *f.PublishedFrom)
	}
	if f.PublishedTo != nil {
		clauses = append(clauses, "p.published_at < ?")
		args = append(args, *f.PublishedTo)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *SQLiteStore) AdjacentPosts(slug string) (prev, next *model.Post, err error) {
	var (
		publishedAt sql.NullTime
		createdAt   time.Time
		currentID   int64
		postType    string
	)
	if err := s.db.QueryRow(
		`SELECT published_at, created_at, id, type FROM posts WHERE slug = ?`, slug,
	).Scan(&publishedAt, &createdAt, &currentID, &postType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get adjacent anchor: %w", err)
	}
	sortTime := createdAt
	if publishedAt.Valid {
		sortTime = publishedAt.Time
	}

	columns := fmt.Sprintf(`SELECT `+postColumnTemplate+`
		FROM posts p LEFT JOIN users u ON u.id = p.author_id`, "''")

	newerQuery := columns + `
		WHERE p.status = 'publish' AND p.type = ?
		  AND (COALESCE(p.published_at, p.created_at) > ?
		       OR (COALESCE(p.published_at, p.created_at) = ? AND p.id > ?))
		ORDER BY COALESCE(p.published_at, p.created_at) ASC, p.id ASC LIMIT 1`

	newer, err := scanPost(s.db.QueryRow(newerQuery, postType, sortTime, sortTime, currentID), false)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("get newer post: %w", err)
	}
	if err == nil {
		prev = newer
	}

	olderQuery := columns + `
		WHERE p.status = 'publish' AND p.type = ?
		  AND (COALESCE(p.published_at, p.created_at) < ?
		       OR (COALESCE(p.published_at, p.created_at) = ? AND p.id < ?))
		ORDER BY COALESCE(p.published_at, p.created_at) DESC, p.id DESC LIMIT 1`

	older, err := scanPost(s.db.QueryRow(olderQuery, postType, sortTime, sortTime, currentID), false)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("get older post: %w", err)
	}
	if err == nil {
		next = older
	}

	return prev, next, nil
}

func (s *SQLiteStore) CreatePost(p *model.Post) error {
	if s.inTx {
		return s.createPost(p)
	}
	tx, err := s.raw.Begin()
	if err != nil {
		return fmt.Errorf("begin create post: %w", err)
	}
	defer tx.Rollback()

	txStore := &SQLiteStore{db: tx, raw: s.raw, inTx: true}
	if err := txStore.createPost(p); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create post: %w", err)
	}
	return nil
}

func (s *SQLiteStore) createPost(p *model.Post) error {
	normalizePost(p)

	res, err := s.db.Exec(
		`INSERT INTO posts
			(slug, title, content, excerpt, cover, type, status, format, template,
			 math, source, source_id, author_id, post_password, allow_comment, pinned,
			 views, likes, published_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.Content, p.Excerpt, p.Cover, p.Type, p.Status,
		p.Format, p.Template, boolInt(p.Math), p.Source, p.SourceID, p.AuthorID,
		p.PostPassword, boolInt(p.AllowComment), boolInt(p.Pinned), p.Views, p.Likes,
		p.PublishedAt, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: slug %q already exists", ErrConflict, p.Slug)
		}
		return fmt.Errorf("create post: %w", err)
	}

	p.ID, _ = res.LastInsertId()

	if p.CategoryIDs != nil {
		if err := replacePostCategories(s.db, p.ID, p.CategoryIDs); err != nil {
			return err
		}
	}
	if p.TagIDs != nil {
		if err := replacePostTags(s.db, p.ID, p.TagIDs); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) UpdatePost(slug string, p *model.Post) error {
	if s.inTx {
		return s.updatePost(slug, p)
	}
	tx, err := s.raw.Begin()
	if err != nil {
		return fmt.Errorf("begin update post: %w", err)
	}
	defer tx.Rollback()

	txStore := &SQLiteStore{db: tx, raw: s.raw, inTx: true}
	if err := txStore.updatePost(slug, p); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update post: %w", err)
	}
	return nil
}

func (s *SQLiteStore) updatePost(slug string, p *model.Post) error {
	normalizePost(p)
	p.UpdatedAt = time.Now()

	var id int64
	if err := s.db.QueryRow(`SELECT id FROM posts WHERE slug = ?`, slug).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("find post: %w", err)
	}

	_, err := s.db.Exec(
		`UPDATE posts SET
			title = ?, content = ?, excerpt = ?, cover = ?, type = ?, status = ?,
			format = ?, template = ?, math = ?,
			author_id = ?, post_password = ?, allow_comment = ?, pinned = ?,
			published_at = CASE
				WHEN ? IS NOT NULL THEN ?
				WHEN ? = 'publish' THEN COALESCE(published_at, ?)
				ELSE published_at END,
			updated_at = ?
		 WHERE id = ?`,
		p.Title, p.Content, p.Excerpt, p.Cover, p.Type, p.Status,
		p.Format, p.Template, boolInt(p.Math),
		p.AuthorID, p.PostPassword, boolInt(p.AllowComment), boolInt(p.Pinned),
		p.PublishedAt, p.PublishedAt, p.Status, time.Now(),
		p.UpdatedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update post %q: %w", slug, err)
	}

	if p.CategoryIDs != nil {
		if err := replacePostCategories(s.db, id, p.CategoryIDs); err != nil {
			return err
		}
	}
	if p.TagIDs != nil {
		if err := replacePostTags(s.db, id, p.TagIDs); err != nil {
			return err
		}
	}

	p.ID = id
	return nil
}

func (s *SQLiteStore) UpdatePostContent(id int64, content, excerpt string) error {
	res, err := s.db.Exec(
		`UPDATE posts SET content = ?, excerpt = ? WHERE id = ?`,
		content, excerpt, id,
	)
	if err != nil {
		return fmt.Errorf("update post content %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeletePost(slug string) error {
	res, err := s.db.Exec(`DELETE FROM posts WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete post %q: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) IncrementPostViews(slug string) error {
	_, err := s.db.Exec(`UPDATE posts SET views = views + 1 WHERE slug = ?`, slug)
	return err
}

func normalizePost(p *model.Post) {
	now := time.Now()

	if p.Type == "" {
		p.Type = model.PostTypePost
	}
	if p.Format == "" {
		p.Format = model.FormatMarkdown
	}
	if p.Status == "" {
		p.Status = model.StatusDraft
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	if p.Status == model.StatusPublish && p.PublishedAt == nil {
		p.PublishedAt = &p.CreatedAt
	}
	if p.Excerpt == "" && p.Content != "" {
		p.Excerpt = model.MakeExcerpt(p.Content, 200)
	}
}

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func replacePostCategories(ex execer, postID int64, ids []int64) error {
	if _, err := ex.Exec(`DELETE FROM post_categories WHERE post_id = ?`, postID); err != nil {
		return fmt.Errorf("clear post categories: %w", err)
	}
	for _, id := range uniqueIDs(ids) {
		if _, err := ex.Exec(
			`INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES (?, ?)`,
			postID, id,
		); err != nil {
			return fmt.Errorf("add post category: %w", err)
		}
	}
	return nil
}

func replacePostTags(ex execer, postID int64, ids []int64) error {
	if _, err := ex.Exec(`DELETE FROM post_tags WHERE post_id = ?`, postID); err != nil {
		return fmt.Errorf("clear post tags: %w", err)
	}
	for _, id := range uniqueIDs(ids) {
		if _, err := ex.Exec(
			`INSERT OR IGNORE INTO post_tags (post_id, tag_id) VALUES (?, ?)`,
			postID, id,
		); err != nil {
			return fmt.Errorf("add post tag: %w", err)
		}
	}
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func ftsPhrase(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}
