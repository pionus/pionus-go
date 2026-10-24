package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func newTestHandlers(t *testing.T) (*arry.Arry, store.Store) {
	t.Helper()

	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	app := arry.New()
	app.Use(middlewares.Auth("secret", http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete))

	r := app.Router()
	r.Get("/api/posts", ListPosts(db))
	r.Get("/api/posts/:slug", GetPost(db))
	r.Post("/api/posts", CreatePost(db))
	r.Put("/api/posts/:slug", UpdatePost(db))
	r.Delete("/api/posts/:slug", DeletePost(db))

	return app, db
}

func doRequest(t *testing.T, app *arry.Arry, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func TestCreatePostRequiresAuth(t *testing.T) {
	app, _ := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "", `{"title":"T","content":"C"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestPostCRUDOverHTTP(t *testing.T) {
	app, db := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"Hello World","content":"# Hi\n\nbody","slug":"hello-world"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	var created model.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != model.StatusPublish || !created.AllowComment {
		t.Fatalf("created = %+v", created)
	}
	if created.Excerpt == "" {
		t.Fatal("excerpt was not generated")
	}

	// Public list sees it.
	rec = doRequest(t, app, http.MethodGet, "/api/posts", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list struct {
		Posts []*model.Post `json:"posts"`
		Total int           `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Posts) != 1 {
		t.Fatalf("list = %+v", list)
	}

	// Make it a draft: public detail must 404, authenticated detail must work.
	rec = doRequest(t, app, http.MethodPut, "/api/posts/hello-world", "secret",
		`{"title":"Hello World","content":"body","status":"draft"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, app, http.MethodGet, "/api/posts/hello-world", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public draft status = %d, want 404", rec.Code)
	}
	rec = doRequest(t, app, http.MethodGet, "/api/posts/hello-world", "secret", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("authed draft status = %d", rec.Code)
	}

	// Delete, then confirm it is gone.
	rec = doRequest(t, app, http.MethodDelete, "/api/posts/hello-world", "secret", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if _, err := db.GetPost("hello-world"); err == nil {
		t.Fatal("post still exists after delete")
	}
}

func TestCreatePostValidatesSlug(t *testing.T) {
	app, _ := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"T","content":"C","slug":"bad/slug"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreatePostGeneratesSlug(t *testing.T) {
	app, _ := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"Hello World","content":"C"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var created model.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Slug != "hello-world" {
		t.Fatalf("slug = %q", created.Slug)
	}
}

func TestUpdatePostKeepsUnspecifiedFields(t *testing.T) {
	app, _ := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"Original","content":"body","slug":"keep","pinned":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, app, http.MethodPut, "/api/posts/keep", "secret",
		`{"title":"Renamed","content":"body2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}

	var updated model.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Renamed" || updated.Slug != "keep" {
		t.Fatalf("updated = %+v", updated)
	}
	if !updated.Pinned {
		t.Fatal("pinned should be preserved when omitted")
	}
	if !updated.AllowComment {
		t.Fatal("allow_comment should be preserved when omitted")
	}
}

func TestPostJSONDoesNotLeakPassword(t *testing.T) {
	app, _ := newTestHandlers(t)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"Secret","content":"body","password":"hunter2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("hunter2")) {
		t.Fatal("post password leaked in JSON response")
	}
}
