package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
)

func TestPostGuard(t *testing.T) {
	guard := NewPostGuard("secret", false)
	post := &model.Post{ID: 7, PostPassword: "pw"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := arry.NewContext(req, rec)
	if guard.Allowed(ctx, post) {
		t.Fatal("post should be locked without a cookie")
	}

	guard.Grant(ctx, post)
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("grant did not set a cookie")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])
	ctx2 := arry.NewContext(req2, httptest.NewRecorder())
	if !guard.Allowed(ctx2, post) {
		t.Fatal("valid cookie should unlock the post")
	}

	changed := &model.Post{ID: 7, PostPassword: "different"}
	if guard.Allowed(ctx2, changed) {
		t.Fatal("cookie should not unlock a post with a different password")
	}
}
