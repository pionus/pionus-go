package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustPost(t *testing.T, s *SQLiteStore, p *model.Post) *model.Post {
	t.Helper()
	if err := s.CreatePost(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPost(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestOptionsSeeded(t *testing.T) {
	s := newTestStore(t)

	opts, err := s.AllOptions()
	if err != nil {
		t.Fatal(err)
	}
	if opts["site.author"] != "Secbone" {
		t.Fatalf("site.author = %q", opts["site.author"])
	}
	if opts["posts.per_page"] != "20" {
		t.Fatalf("posts.per_page = %q", opts["posts.per_page"])
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s1, err := NewSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s1.db.Exec(`INSERT INTO options (name, value) VALUES ('x', 'y')`); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := NewSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	var rows, distinct int
	if err := s2.db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT version) FROM schema_migrations`).Scan(&rows, &distinct); err != nil {
		t.Fatal(err)
	}
	if rows != distinct {
		t.Fatalf("migrations applied more than once: rows=%d distinct=%d", rows, distinct)
	}
	if distinct < 2 {
		t.Fatalf("expected both migrations to be applied, got %d", distinct)
	}
}

func TestPostLifecycle(t *testing.T) {
	s := newTestStore(t)

	cat := &model.Category{Name: "Go", Slug: "go"}
	if err := s.CreateCategory(cat); err != nil {
		t.Fatal(err)
	}
	tag := &model.Tag{Name: "SQLite", Slug: "sqlite"}
	if err := s.CreateTag(tag); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	p := &model.Post{
		Slug:        "hello-world",
		Title:       "Hello World",
		Content:     "# Heading\n\nThis is **markdown** content.",
		Type:        model.PostTypePost,
		Status:      model.StatusPublish,
		PublishedAt: &at,
		CategoryIDs: []int64{cat.ID},
		TagIDs:      []int64{tag.ID},
	}
	got := mustPost(t, s, p)

	if got.Author != "Secbone" {
		t.Fatalf("author = %q, want Secbone", got.Author)
	}
	if got.Excerpt == "" {
		t.Fatal("excerpt was not generated")
	}
	if len(got.Categories) != 1 || got.Categories[0].Slug != "go" {
		t.Fatalf("categories = %+v", got.Categories)
	}
	if len(got.Tags) != 1 || got.Tags[0].Slug != "sqlite" {
		t.Fatalf("tags = %+v", got.Tags)
	}

	list, total, err := s.ListPosts(PostFilter{Type: model.PostTypePost, Statuses: []string{model.StatusPublish}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("total=%d len=%d", total, len(list))
	}
	if list[0].Content != "" {
		t.Fatal("list should not load content by default")
	}

	byTag, _, err := s.ListPosts(PostFilter{TagSlug: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTag) != 1 {
		t.Fatalf("tag filter returned %d posts", len(byTag))
	}

	byQuery, _, err := s.ListPosts(PostFilter{Query: "markdown"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byQuery) != 1 {
		t.Fatalf("query filter returned %d posts", len(byQuery))
	}

	// Update: rename, tag removal and a draft status.
	got.Title = "Hello Again"
	got.Status = model.StatusDraft
	got.TagIDs = []int64{}
	if err := s.UpdatePost(p.Slug, got); err != nil {
		t.Fatal(err)
	}

	updated, err := s.GetPost(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Hello Again" || updated.Status != model.StatusDraft {
		t.Fatalf("update did not apply: %+v", updated)
	}
	if len(updated.Tags) != 0 {
		t.Fatalf("tags not cleared: %+v", updated.Tags)
	}
	if len(updated.Categories) != 1 {
		t.Fatalf("categories should be unchanged: %+v", updated.Categories)
	}

	if err := s.DeletePost(p.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPost(p.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestPostSlugConflict(t *testing.T) {
	s := newTestStore(t)

	p := &model.Post{Slug: "dup", Title: "A", Content: "a", Status: model.StatusPublish}
	if err := s.CreatePost(p); err != nil {
		t.Fatal(err)
	}
	dup := &model.Post{Slug: "dup", Title: "B", Content: "b", Status: model.StatusPublish}
	if err := s.CreatePost(dup); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestAdjacentPostsTieBreak(t *testing.T) {
	s := newTestStore(t)

	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	posts := []*model.Post{
		{Slug: "a", Title: "A", Content: "a", Status: model.StatusPublish, PublishedAt: &same},
		{Slug: "b", Title: "B", Content: "b", Status: model.StatusPublish, PublishedAt: &same},
		{Slug: "c", Title: "C", Content: "c", Status: model.StatusPublish, PublishedAt: &same},
	}
	for _, p := range posts {
		if err := s.CreatePost(p); err != nil {
			t.Fatal(err)
		}
	}

	prev, next, err := s.AdjacentPosts("b")
	if err != nil {
		t.Fatal(err)
	}
	if prev == nil || prev.Slug != "c" {
		t.Fatalf("prev = %+v, want c", prev)
	}
	if next == nil || next.Slug != "a" {
		t.Fatalf("next = %+v, want a", next)
	}
}

func TestCategoriesAndTags(t *testing.T) {
	s := newTestStore(t)

	c := &model.Category{Name: "Tech", Slug: "tech"}
	if err := s.CreateCategory(c); err != nil {
		t.Fatal(err)
	}
	c.Name = "Technology"
	c.SortOrder = 2
	if err := s.UpdateCategory("tech", c); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCategory("tech")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Technology" || got.SortOrder != 2 {
		t.Fatalf("category = %+v", got)
	}

	tag, err := s.UpsertTag("Go Lang", "")
	if err != nil {
		t.Fatal(err)
	}
	if tag.Slug != "go-lang" {
		t.Fatalf("slug = %q, want go-lang", tag.Slug)
	}
	again, err := s.UpsertTag("Go Lang", "")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != tag.ID {
		t.Fatalf("UpsertTag created a duplicate: %d != %d", again.ID, tag.ID)
	}
}

func TestComments(t *testing.T) {
	s := newTestStore(t)

	p := mustPost(t, s, &model.Post{Slug: "post", Title: "Post", Content: "body", Status: model.StatusPublish})

	c := &model.Comment{PostID: p.ID, Author: "Reader", Content: "Nice post"}
	if err := s.CreateComment(c); err != nil {
		t.Fatal(err)
	}
	if c.Status != model.CommentPending {
		t.Fatalf("status = %q", c.Status)
	}

	list, total, err := s.ListComments(CommentFilter{PostID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].PostTitle != "Post" {
		t.Fatalf("comments = %+v total=%d", list, total)
	}

	if err := s.UpdateCommentStatus(c.ID, model.CommentApproved); err != nil {
		t.Fatal(err)
	}
	n, err := s.CountComments(p.ID, model.CommentApproved)
	if err != nil || n != 1 {
		t.Fatalf("approved count = %d, err = %v", n, err)
	}
}

func TestUsersAndSessions(t *testing.T) {
	s := newTestStore(t)

	u := &model.User{Username: "secbone", Email: "me@example.com", Role: model.RoleOwner}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetUserByUsername("secbone")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName() != "secbone" || !got.CanManage() {
		t.Fatalf("user = %+v", got)
	}

	now := time.Now()
	sess := &model.Session{
		TokenHash: "hash",
		UserID:    u.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}

	_, sessionUser, err := s.GetSession("hash", now)
	if err != nil {
		t.Fatal(err)
	}
	if sessionUser.ID != u.ID {
		t.Fatalf("session user = %d", sessionUser.ID)
	}

	if _, _, err := s.GetSession("hash", now.Add(2*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session err = %v", err)
	}

	if err := s.TouchUserLogin(u.ID, now); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetUser(u.ID)
	if got.LastLoginAt == nil {
		t.Fatal("last_login_at was not set")
	}
}

func TestMediaAndRedirects(t *testing.T) {
	s := newTestStore(t)

	m := &model.Media{Path: "2026/10/pic.png", Filename: "pic.png", MIME: "image/png", Size: 42}
	if err := s.CreateMedia(m); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.ListMedia(1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("media total=%d len=%d err=%v", total, len(list), err)
	}

	if err := s.SetRedirect("/archives/1/", "/article/hello/", 301); err != nil {
		t.Fatal(err)
	}
	r, err := s.FindRedirect("/archives/1/")
	if err != nil {
		t.Fatal(err)
	}
	if r.To != "/article/hello/" || r.Status != 301 {
		t.Fatalf("redirect = %+v", r)
	}
}

func TestPublishedBeforeFiltersScheduledPosts(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	for _, p := range []*model.Post{
		{Slug: "past", Title: "Past", Content: "ok", Status: model.StatusPublish, PublishedAt: &past},
		{Slug: "future", Title: "Future", Content: "ok", Status: model.StatusPublish, PublishedAt: &future},
		{Slug: "draft", Title: "Draft", Content: "ok", Status: model.StatusDraft},
	} {
		if err := s.CreatePost(p); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.ListPosts(PostFilter{
		Statuses:        []string{model.StatusPublish},
		PublishedBefore: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].Slug != "past" {
		t.Fatalf("filtered list total=%d list=%+v", total, list)
	}

	// Ordering: newest published first.
	list, _, err = s.ListPosts(PostFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Slug != "future" {
		t.Fatalf("order = %+v", list)
	}
}
