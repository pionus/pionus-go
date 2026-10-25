package typechoredirects

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/plugin"
	"github.com/pionus/pionus-go/internal/store"
)

func newRedirectApp(t *testing.T) *arry.Arry {
	t.Helper()

	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	category := &model.Category{Name: "Tech", Slug: "tech"}
	if err := db.CreateCategory(category); err != nil {
		t.Fatal(err)
	}
	tag := &model.Tag{Name: "Go", Slug: "go"}
	if err := db.CreateTag(tag); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "new-post", Title: "New", Content: "body", Status: model.StatusPublish,
		Source: "typecho", SourceID: "10", TagIDs: []int64{tag.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "about", Title: "About", Content: "body", Type: model.PostTypePage,
		Status: model.StatusPublish, Source: "typecho", SourceID: "12",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "draft-share", Title: "Draft Share", Content: "body",
		Status: model.StatusDraft, Source: "typecho", SourceID: "99",
	}); err != nil {
		t.Fatal(err)
	}

	app := arry.New()
	p := New().(*Plugin)
	if err := p.Init(nil, &plugin.Env{Store: db, App: app}); err != nil {
		t.Fatal(err)
	}
	app.Router().Get("/*", func(ctx arry.Context) {
		ctx.Text(http.StatusOK, "ok")
	})
	return app
}

func TestTypechoRedirects(t *testing.T) {
	app := newRedirectApp(t)

	tests := []struct {
		path     string
		location string
	}{
		{"/archives/10", "/article/new-post"},
		{"/archives/10/", "/article/new-post"},
		{"/index.php/archives/10/", "/article/new-post"},
		{"/?p=10", "/article/new-post"},
		{"/index.php?p=10", "/article/new-post"},
		{"/about.html", "/article/about"},
		{"/index.php/about.html", "/article/about"},
		{"/category/tech/", "/category/tech"},
		{"/index.php/tag/go/", "/tag/go"},
		{"/usr/uploads/2015/a.jpg", "/uploads/2015/a.jpg"},
		{"/search/hello%20world/", "/search?q=hello+world"},
		{"/feed/atom/", "/feed"},
		{"/index.php", "/"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently {
			t.Fatalf("%s status = %d", tt.path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != tt.location {
			t.Fatalf("%s location = %q, want %q", tt.path, got, tt.location)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/article/new-post", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("new URL = %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/archives/99", nil)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("draft legacy URL = %d, want 200 fallback (no redirect)", rec.Code)
	}
}
