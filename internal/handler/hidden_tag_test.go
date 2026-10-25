package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
	"github.com/pionus/pionus-go/web"
)

type hiddenTagFixture struct {
	app   *arry.Arry
	db    *store.SQLiteStore
	guard *PostGuard
	tag   *model.Tag
}

func newHiddenTagFixture(t *testing.T) *hiddenTagFixture {
	t.Helper()

	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	hash, err := auth.HashPassword("tag-pass")
	if err != nil {
		t.Fatal(err)
	}
	tag := &model.Tag{Name: "网安", Slug: "security", Hidden: true, PasswordHash: hash}
	if err := db.CreateTag(tag); err != nil {
		t.Fatal(err)
	}
	publicTag := &model.Tag{Name: "公开", Slug: "public"}
	if err := db.CreateTag(publicTag); err != nil {
		t.Fatal(err)
	}

	if err := db.CreatePost(&model.Post{
		Slug: "secret-post", Title: "Secret Hidden", Content: "top secret body",
		Status: model.StatusPublish, TagIDs: []int64{tag.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreatePost(&model.Post{
		Slug: "public-post", Title: "Public Post", Content: "public body",
		Status: model.StatusPublish, TagIDs: []int64{publicTag.ID},
	}); err != nil {
		t.Fatal(err)
	}
	secretPost, err := db.GetPost("secret-post")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateComment(&model.Comment{
		PostID: secretPost.ID, Author: "Reader", Content: "secret comment",
		Status: model.CommentApproved,
	}); err != nil {
		t.Fatal(err)
	}

	guard := NewPostGuard("test-secret", false)
	deps := NewDeps(db)
	app := arry.New()
	app.ViewsWithConfig(web.EngineConfig())
	app.Router().Get("/", Index(deps))
	app.Router().Get("/article/:slug", ArticleDetail(deps, guard))
	app.Router().Post("/article/:slug/unlock", UnlockPost(deps, guard))
	app.Router().Get("/tag/:slug", TagArchive(deps, guard))
	app.Router().Post("/tag/:slug/unlock", UnlockTag(deps, guard))
	app.Router().Get("/tags", TagsPage(deps))
	app.Router().Get("/article", ArticleList(deps))
	app.Router().Get("/search", Search(deps))
	app.Router().Get("/feed", Feed(deps))
	app.Router().Get("/feed/comments", CommentsFeed(deps))
	app.Router().Get("/sitemap.xml", Sitemap(deps))
	app.Router().Get("/api/posts", ListPosts(deps))
	app.Router().Get("/api/posts/:slug", GetPost(deps, guard))

	return &hiddenTagFixture{app: app, db: db, guard: guard, tag: tag}
}

func hiddenTagRequest(t *testing.T, app *arry.Arry, method, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func findingTagCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if strings.HasPrefix(cookie.Name, "pionus_tag_") {
			return cookie
		}
	}
	t.Fatal("tag cookie not set")
	return nil
}

func TestHiddenTagHidesAndGatesPost(t *testing.T) {
	f := newHiddenTagFixture(t)

	home := hiddenTagRequest(t, f.app, http.MethodGet, "/", nil, nil)
	if home.Code != http.StatusOK || strings.Contains(home.Body.String(), "Secret Hidden") || !strings.Contains(home.Body.String(), "Public Post") {
		t.Fatalf("home status=%d body=%s", home.Code, home.Body.String())
	}

	feed := hiddenTagRequest(t, f.app, http.MethodGet, "/feed", nil, nil)
	if strings.Contains(feed.Body.String(), "Secret Hidden") {
		t.Fatalf("feed leaked hidden post: %s", feed.Body.String())
	}
	commentsFeed := hiddenTagRequest(t, f.app, http.MethodGet, "/feed/comments", nil, nil)
	if strings.Contains(commentsFeed.Body.String(), "secret comment") {
		t.Fatalf("comments feed leaked hidden post: %s", commentsFeed.Body.String())
	}
	archive := hiddenTagRequest(t, f.app, http.MethodGet, "/article", nil, nil)
	if strings.Contains(archive.Body.String(), "Secret Hidden") {
		t.Fatalf("archive leaked hidden post: %s", archive.Body.String())
	}
	search := hiddenTagRequest(t, f.app, http.MethodGet, "/search?q=top+secret", nil, nil)
	if strings.Contains(search.Body.String(), "Secret Hidden") {
		t.Fatalf("search leaked hidden post: %s", search.Body.String())
	}
	sitemap := hiddenTagRequest(t, f.app, http.MethodGet, "/sitemap.xml", nil, nil)
	if strings.Contains(sitemap.Body.String(), "/article/secret-post") || strings.Contains(sitemap.Body.String(), "/tag/security") {
		t.Fatalf("sitemap leaked hidden content: %s", sitemap.Body.String())
	}
	tagsPage := hiddenTagRequest(t, f.app, http.MethodGet, "/tags", nil, nil)
	if strings.Contains(tagsPage.Body.String(), "网安") || !strings.Contains(tagsPage.Body.String(), "公开") {
		t.Fatalf("tags page = %s", tagsPage.Body.String())
	}

	list := hiddenTagRequest(t, f.app, http.MethodGet, "/api/posts", nil, nil)
	var listBody struct {
		Posts []*model.Post `json:"posts"`
		Total int           `json:"total"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if listBody.Total != 1 || len(listBody.Posts) != 1 || listBody.Posts[0].Slug != "public-post" {
		t.Fatalf("api list = %+v", listBody)
	}

	article := hiddenTagRequest(t, f.app, http.MethodGet, "/article/secret-post", nil, nil)
	if article.Code != http.StatusOK || strings.Contains(article.Body.String(), "top secret body") ||
		!strings.Contains(article.Body.String(), "这篇文章需要密码") {
		t.Fatalf("article status=%d body=%s", article.Code, article.Body.String())
	}

	setupAPI := hiddenTagRequest(t, f.app, http.MethodGet, "/api/posts/secret-post", nil, nil)
	if setupAPI.Code != http.StatusForbidden {
		t.Fatalf("api hidden status = %d", setupAPI.Code)
	}

	wrong := hiddenTagRequest(t, f.app, http.MethodPost, "/article/secret-post/unlock",
		url.Values{"password": {"wrong"}}, nil)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", wrong.Code)
	}

	unlock := hiddenTagRequest(t, f.app, http.MethodPost, "/article/secret-post/unlock",
		url.Values{"password": {"tag-pass"}}, nil)
	if unlock.Code != http.StatusSeeOther {
		t.Fatalf("unlock status = %d body=%s", unlock.Code, unlock.Body.String())
	}
	cookie := findingTagCookie(t, unlock)

	unlocked := hiddenTagRequest(t, f.app, http.MethodGet, "/article/secret-post", nil, cookie)
	if unlocked.Code != http.StatusOK || !strings.Contains(unlocked.Body.String(), "top secret body") {
		t.Fatalf("unlocked article status=%d body=%s", unlocked.Code, unlocked.Body.String())
	}

	api := hiddenTagRequest(t, f.app, http.MethodGet, "/api/posts/secret-post", nil, cookie)
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), "top secret body") {
		t.Fatalf("api unlocked status=%d body=%s", api.Code, api.Body.String())
	}
	if strings.Contains(api.Body.String(), "password_hash") {
		t.Fatalf("api leaked tag password hash: %s", api.Body.String())
	}

	// Unlocking one post must not make it appear in public listings.
	home = hiddenTagRequest(t, f.app, http.MethodGet, "/", nil, cookie)
	if strings.Contains(home.Body.String(), "Secret Hidden") {
		t.Fatalf("unlocked post leaked into home: %s", home.Body.String())
	}
	list = hiddenTagRequest(t, f.app, http.MethodGet, "/api/posts", nil, cookie)
	if strings.Contains(list.Body.String(), "secret-post") {
		t.Fatalf("unlocked post leaked into api list: %s", list.Body.String())
	}
}

func TestHiddenTagArchiveRequiresPassword(t *testing.T) {
	f := newHiddenTagFixture(t)

	before := hiddenTagRequest(t, f.app, http.MethodGet, "/tag/security", nil, nil)
	if before.Code != http.StatusOK || !strings.Contains(before.Body.String(), "这个标签下的文章需要密码") {
		t.Fatalf("tag page status=%d body=%s", before.Code, before.Body.String())
	}

	wrong := hiddenTagRequest(t, f.app, http.MethodPost, "/tag/security/unlock",
		url.Values{"password": {"wrong"}}, nil)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong tag password status = %d", wrong.Code)
	}

	unlock := hiddenTagRequest(t, f.app, http.MethodPost, "/tag/security/unlock",
		url.Values{"password": {"tag-pass"}}, nil)
	if unlock.Code != http.StatusSeeOther {
		t.Fatalf("tag unlock status = %d body=%s", unlock.Code, unlock.Body.String())
	}
	cookie := findingTagCookie(t, unlock)

	after := hiddenTagRequest(t, f.app, http.MethodGet, "/tag/security", nil, cookie)
	if after.Code != http.StatusOK || !strings.Contains(after.Body.String(), "Secret Hidden") {
		t.Fatalf("unlocked tag page status=%d body=%s", after.Code, after.Body.String())
	}
}
