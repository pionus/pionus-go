package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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

func TestPreviewToken(t *testing.T) {
	guard := NewPostGuard("secret", false)
	post := &model.Post{ID: 9, Slug: "draft", UpdatedAt: time.Now()}
	token := guard.PreviewToken(post, time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/article/draft?preview="+url.QueryEscape(token), nil)
	ctx := arry.NewContext(req, httptest.NewRecorder())
	if !guard.ValidPreview(ctx, post) {
		t.Fatal("valid preview token rejected")
	}

	changed := *post
	changed.UpdatedAt = post.UpdatedAt.Add(time.Second)
	if guard.ValidPreview(ctx, &changed) {
		t.Fatal("preview token should be invalid after an edit")
	}

	expired := guard.PreviewToken(post, -time.Hour)
	req = httptest.NewRequest(http.MethodGet, "/article/draft?preview="+url.QueryEscape(expired), nil)
	ctx = arry.NewContext(req, httptest.NewRecorder())
	if guard.ValidPreview(ctx, post) {
		t.Fatal("expired preview token accepted")
	}
}

func TestTagGuard(t *testing.T) {
	guard := NewPostGuard("secret", false)
	tag := &model.Tag{ID: 12, Hidden: true, PasswordHash: "hash-v1"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := arry.NewContext(req, rec)
	if guard.AllowedTag(ctx, tag) {
		t.Fatal("tag should be locked without a cookie")
	}

	guard.GrantTag(ctx, tag)
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("tag grant did not set a cookie")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(cookies[0])
	ctx2 := arry.NewContext(req2, httptest.NewRecorder())
	if !guard.AllowedTag(ctx2, tag) {
		t.Fatal("valid tag cookie should unlock the tag")
	}
	if !guard.AllowedAnyTag(ctx2, []*model.Tag{tag}) {
		t.Fatal("AllowedAnyTag rejected a valid tag cookie")
	}

	changed := &model.Tag{ID: 12, Hidden: true, PasswordHash: "hash-v2"}
	if guard.AllowedTag(ctx2, changed) {
		t.Fatal("tag cookie should not survive a password change")
	}
	if guard.AllowedTag(ctx2, &model.Tag{ID: 12, Hidden: false, PasswordHash: "hash-v1"}) {
		t.Fatal("non-hidden tag should not be guarded")
	}
}
