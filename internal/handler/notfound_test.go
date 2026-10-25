package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/web"
)

func TestNotFoundIsThemed(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := arry.New()
	app.ViewsWithConfig(web.EngineConfig())
	app.Router().Get("/*", NotFound(NewDeps(db)))

	req := httptest.NewRequest(http.MethodGet, "/index.php/archives/1/", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); body == "" || !strings.Contains(body, "页面没找到") {
		t.Fatalf("themed 404 not rendered")
	}
}
