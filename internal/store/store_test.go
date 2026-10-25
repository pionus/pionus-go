package store

import (
	"context"
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

func TestWithTx(t *testing.T) {
	s := newTestStore(t)

	if err := s.WithTx(func(tx Store) error {
		return tx.CreatePost(&model.Post{Slug: "tx-post", Title: "TX", Content: "body", Status: model.StatusPublish})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPost("tx-post"); err != nil {
		t.Fatal("committed transaction post missing")
	}

	wantErr := errors.New("rollback")
	if err := s.WithTx(func(tx Store) error {
		if err := tx.CreatePost(&model.Post{Slug: "rolled-back", Title: "No", Content: "body", Status: model.StatusPublish}); err != nil {
			return err
		}
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want rollback error", err)
	}
	if _, err := s.GetPost("rolled-back"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rolled-back post exists: %v", err)
	}
}

func TestPing(t *testing.T) {
	s := newTestStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
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

func TestOptionsCacheInvalidates(t *testing.T) {
	s := newTestStore(t)
	if v, _ := s.GetOption("site.title"); v != "Secbone's Blog" {
		t.Fatalf("initial title = %q", v)
	}
	if err := s.SetOption("site.title", "New Title"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetOption("site.title"); v != "New Title" {
		t.Fatalf("cached title = %q", v)
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
	if distinct < 1 {
		t.Fatalf("expected migrations to be applied, got %d", distinct)
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
		Math:        true,
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
	if !got.Math {
		t.Fatal("math flag was not persisted")
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
	listWithTaxonomy, _, err := s.ListPosts(PostFilter{WithTaxonomy: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(listWithTaxonomy[0].Categories) != 1 || len(listWithTaxonomy[0].Tags) != 1 {
		t.Fatalf("batch taxonomy = %+v", listWithTaxonomy[0])
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
	got.Math = false
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
	if updated.Math {
		t.Fatal("math flag was not cleared")
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

func TestMedia(t *testing.T) {
	s := newTestStore(t)

	m := &model.Media{Path: "2026/10/pic.png", Filename: "pic.png", MIME: "image/png", Size: 42}
	if err := s.CreateMedia(m); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.ListMedia(1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("media total=%d len=%d err=%v", total, len(list), err)
	}
}

func TestFTSSearch(t *testing.T) {
	s := newTestStore(t)
	posts := []*model.Post{
		{Slug: "sql", Title: "SQL注入的几点误区", Content: "数据库安全内容", Status: model.StatusPublish},
		{Slug: "hello", Title: "Hello World", Content: "plain english body", Status: model.StatusPublish},
	}
	for _, p := range posts {
		if err := s.CreatePost(p); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		query string
		want  int
	}{
		{"SQL注入", 1},
		{"注入", 1},      // 2 chars: LIKE fallback
		{"english", 1}, // 7 chars: FTS
		{"missing", 0},
	}
	for _, tc := range tests {
		list, total, err := s.ListPosts(PostFilter{Query: tc.query})
		if err != nil {
			t.Fatalf("%q: %v", tc.query, err)
		}
		if total != tc.want || len(list) != tc.want {
			t.Fatalf("%q: total=%d list=%d, want %d", tc.query, total, len(list), tc.want)
		}
	}

	// Content updates must refresh the FTS index.
	got, err := s.GetPost("sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePostContent(got.ID, "完全不同的内容", "完全不同的内容"); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := s.ListPosts(PostFilter{Query: "数据库安全"}); total != 0 {
		t.Fatalf("old FTS content still matches")
	}
	if _, total, _ := s.ListPosts(PostFilter{Query: "完全不同"}); total != 1 {
		t.Fatalf("new FTS content does not match")
	}
}

func TestNotes(t *testing.T) {
	s := newTestStore(t)

	n := &model.Note{Content: "第一条微语", Source: "test", SourceID: "1"}
	if err := s.CreateNote(n); err != nil {
		t.Fatal(err)
	}
	if n.ID == 0 || n.Status != model.StatusPublish {
		t.Fatalf("note = %+v", n)
	}

	got, err := s.GetNoteBySource("test", "1")
	if err != nil || got.ID != n.ID {
		t.Fatalf("get note: %+v %v", got, err)
	}

	list, total, err := s.ListNotes(NoteFilter{Statuses: []string{model.StatusPublish}})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("list notes total=%d len=%d err=%v", total, len(list), err)
	}
	count, err := s.CountNotes("")
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}

	if err := s.UpdateNote(n.ID, "修改后的微语", model.StatusHidden); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetNote(n.ID)
	if err != nil || updated.Content != "修改后的微语" || updated.Status != model.StatusHidden {
		t.Fatalf("updated note = %+v err=%v", updated, err)
	}

	if err := s.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNoteBySource("test", "1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete err=%v", err)
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

func TestHiddenTagVisibility(t *testing.T) {
	s := newTestStore(t)

	hidden := &model.Tag{Name: "网安", Slug: "security", Hidden: true, PasswordHash: "hash-v1"}
	public := &model.Tag{Name: "公开", Slug: "public"}
	if err := s.CreateTag(hidden); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTag(public); err != nil {
		t.Fatal(err)
	}

	older := mustPost(t, s, &model.Post{
		Slug: "older", Title: "Older", Content: "older", Status: model.StatusPublish,
		TagIDs: []int64{public.ID},
	})
	secret := mustPost(t, s, &model.Post{
		Slug: "secret", Title: "Secret", Content: "secret", Status: model.StatusPublish,
		TagIDs: []int64{hidden.ID},
	})
	newer := mustPost(t, s, &model.Post{
		Slug: "newer", Title: "Newer", Content: "newer", Status: model.StatusPublish,
		TagIDs: []int64{public.ID},
	})
	mixed := mustPost(t, s, &model.Post{
		Slug: "mixed", Title: "Mixed", Content: "mixed", Status: model.StatusPublish,
		TagIDs: []int64{public.ID, hidden.ID},
	})

	all, total, err := s.ListPosts(PostFilter{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(all) != 4 {
		t.Fatalf("admin list total=%d len=%d", total, len(all))
	}

	visible, total, err := s.ListPosts(PostFilter{Page: 1, Limit: 20, ExcludeHiddenTagged: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(visible) != 2 {
		t.Fatalf("public list total=%d len=%d", total, len(visible))
	}
	for _, post := range visible {
		if post.Slug == secret.Slug || post.Slug == mixed.Slug {
			t.Fatalf("hidden post leaked into public list: %s", post.Slug)
		}
	}

	got, err := s.GetPost(secret.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 1 || !got.Tags[0].Hidden || got.Tags[0].PasswordHash != "hash-v1" {
		t.Fatalf("secret tags = %+v", got.Tags)
	}

	prev, next, err := s.AdjacentPosts(secret.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if prev == nil || prev.Slug != newer.Slug {
		t.Fatalf("prev = %+v, want %s", prev, newer.Slug)
	}
	if next == nil || next.Slug != older.Slug {
		t.Fatalf("next = %+v, want %s", next, older.Slug)
	}

	if err := s.CreateComment(&model.Comment{
		PostID: secret.ID, Author: "Reader", Content: "hidden comment", Status: model.CommentApproved,
	}); err != nil {
		t.Fatal(err)
	}
	comments, _, err := s.ListComments(CommentFilter{PostID: secret.ID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || !comments[0].PostHidden {
		t.Fatalf("comment = %+v", comments)
	}

	hidden.PasswordHash = "hash-v2"
	hidden.Hidden = false
	if err := s.UpdateTag(hidden.Slug, hidden); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetTag(hidden.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Hidden || updated.PasswordHash != "hash-v2" {
		t.Fatalf("updated tag = %+v", updated)
	}

	reused, err := s.UpsertTag("网安", "")
	if err != nil {
		t.Fatal(err)
	}
	if reused.ID != hidden.ID {
		t.Fatalf("name lookup created duplicate tag: %+v", reused)
	}
}
