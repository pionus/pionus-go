package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/store"
)

type fakeOptions map[string]string

func (f fakeOptions) GetOption(name string) (string, error) {
	if value, ok := f[name]; ok {
		return value, nil
	}
	return "", store.ErrNotFound
}

// TestRobotsWithNarrowDeps proves that a public handler can be exercised with
// only the interface it needs instead of a full SQLite store.
func TestRobotsWithNarrowDeps(t *testing.T) {
	app := arry.New()
	app.Router().Get("/robots.txt", Robots(Deps{
		Options: fakeOptions{"site.url": "https://example.com"},
	}))

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sitemap: https://example.com/sitemap.xml") {
		t.Fatalf("robots = %s", rec.Body.String())
	}
}
