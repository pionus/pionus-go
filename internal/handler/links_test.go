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
	"github.com/pionus/pionus-go/web"
)

func TestFriendLinksPageRendersManagedLinks(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.CreatePost(&model.Post{
		Slug: "links", Title: "传送门", Content: "这里是一些常去的朋友和站点。",
		Type: model.PostTypePage, Status: model.StatusPublish,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateFriendLink(&model.FriendLink{
		Name: "Hidden Friend", URL: "https://hidden.example/", Visible: false,
	}); err != nil {
		t.Fatal(err)
	}

	app := arry.New()
	app.ViewsWithConfig(web.EngineConfig())
	app.Router().Get("/article/:slug", ArticleDetail(NewDeps(db), NewPostGuard("secret", false)))

	req := httptest.NewRequest(http.MethodGet, "/article/links", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"这里是一些常去的朋友和站点", "Akawa", "blog.akawa.ink", "github.com/qar"} {
		if !strings.Contains(body, want) {
			t.Fatalf("links page missing %q", want)
		}
	}
	if strings.Contains(body, "Hidden Friend") {
		t.Fatal("hidden friend link leaked")
	}
}
