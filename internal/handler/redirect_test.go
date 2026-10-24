package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/web"
)

func TestRedirectHandler(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SetRedirect("/index.php/archives/42/", "/article/archives-42", 301); err != nil {
		t.Fatal(err)
	}
	if err := db.SetRedirect("/archives/42/", "/article/archives-42", 301); err != nil {
		t.Fatal(err)
	}
	if err := db.SetRedirect("/links.html", "/article/links", 301); err != nil {
		t.Fatal(err)
	}

	app := arry.New()
	app.ViewsWithConfig(web.EngineConfig())
	app.Router().Get("/*", Redirects(db))

	cases := []struct {
		path     string
		status   int
		location string
	}{
		{"/index.php/archives/42/", 301, "/article/archives-42"},
		{"/index.php/archives/42", 301, "/article/archives-42"},
		{"/archives/42/", 301, "/article/archives-42"},
		{"/links.html", 301, "/article/links"},
		{"/missing", 404, ""},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want %d", tc.path, rec.Code, tc.status)
		}
		if tc.location != "" && rec.Header().Get("Location") != tc.location {
			t.Errorf("%s: location = %q, want %q", tc.path, rec.Header().Get("Location"), tc.location)
		}
	}
}

func TestRedirectPreservesQuery(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SetRedirect("/old/", "/new", 301); err != nil {
		t.Fatal(err)
	}

	app := arry.New()
	app.ViewsWithConfig(web.EngineConfig())
	app.Router().Get("/*", Redirects(db))

	req := httptest.NewRequest(http.MethodGet, "/old/?page=2", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != 301 {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/new?page=2" {
		t.Fatalf("location = %q", loc)
	}
}
