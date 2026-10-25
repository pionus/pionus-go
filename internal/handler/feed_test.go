package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func newFeedStore(t *testing.T) store.Store {
	t.Helper()
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func feedRequest(app *arry.Arry, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func TestRobotsAndSitemap(t *testing.T) {
	db := newFeedStore(t)
	if err := db.SetOption("site.url", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "post", Title: "Post", Content: "body",
		Status: model.StatusPublish,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateNote(&model.Note{Content: "note"}); err != nil {
		t.Fatal(err)
	}

	deps := NewDeps(db)
	app := arry.New()
	app.Router().Get("/robots.txt", Robots(deps))
	app.Router().Get("/sitemap.xml", Sitemap(deps))

	robots := feedRequest(app, "/robots.txt")
	if robots.Code != http.StatusOK {
		t.Fatalf("robots status = %d", robots.Code)
	}
	for _, want := range []string{"Disallow: /admin/", "Disallow: /search", "Sitemap: https://example.com/sitemap.xml"} {
		if !strings.Contains(robots.Body.String(), want) {
			t.Fatalf("robots missing %q: %s", want, robots.Body.String())
		}
	}

	sitemap := feedRequest(app, "/sitemap.xml")
	if sitemap.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d", sitemap.Code)
	}
	for _, want := range []string{"https://example.com/notes", "https://example.com/article/post"} {
		if !strings.Contains(sitemap.Body.String(), want) {
			t.Fatalf("sitemap missing %q", want)
		}
	}
}

func TestNotesFeed(t *testing.T) {
	db := newFeedStore(t)
	if err := db.SetOption("site.url", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateNote(&model.Note{Content: "一条微语"}); err != nil {
		t.Fatal(err)
	}

	deps := NewDeps(db)
	app := arry.New()
	app.Router().Get("/feed/notes", NotesFeed(deps))
	rec := feedRequest(app, "/feed/notes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "一条微语") || !strings.Contains(body, "/notes/") {
		t.Fatalf("notes feed = %s", body)
	}
}

func TestFeedSkipsPasswordPosts(t *testing.T) {
	db := newFeedStore(t)
	if err := db.SetOption("site.url", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "public", Title: "Public", Content: "body", Status: model.StatusPublish,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "secret", Title: "Secret", Content: "body", Status: model.StatusPublish,
		PostPassword: "pw",
	}); err != nil {
		t.Fatal(err)
	}

	deps := NewDeps(db)
	app := arry.New()
	app.Router().Get("/feed", Feed(deps))

	rec := feedRequest(app, "/feed")
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Public") || strings.Contains(body, "Secret") {
		t.Fatalf("feed leaked or missed posts: %s", body)
	}
}
