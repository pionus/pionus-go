package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/auth"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

type testEnv struct {
	server *httptest.Server
	client *http.Client
	store  *store.SQLiteStore
	base   string
	jar    *cookiejar.Jar
}

func newTestEnv(t *testing.T) *testEnv {
	return newTestEnvWithEvents(t, events.Nop{})
}

func newTestEnvWithEvents(t *testing.T, emitter events.Emitter) *testEnv {
	t.Helper()

	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	manager := auth.NewManager(db, "test_session", time.Hour, false)
	a := New(Options{
		Store:       db,
		Auth:        manager,
		Events:      emitter,
		UploadDir:   t.TempDir(),
		MaxUploadMB: 1,
	})

	app := arry.New()
	app.Use(manager.Middleware())
	a.Register(app)

	server := httptest.NewServer(app)
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &testEnv{
		server: server,
		client: &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		store: db,
		base:  server.URL,
		jar:   jar,
	}
}

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := e.client.Get(e.base + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) csrf(t *testing.T) string {
	t.Helper()

	e.get(t, "/admin/login").Body.Close()
	u, _ := url.Parse(e.base)
	for _, c := range e.jar.Cookies(u) {
		if c.Name == "pionus_csrf" {
			return c.Value
		}
	}
	t.Fatal("csrf cookie not set")
	return ""
}

func (e *testEnv) postForm(t *testing.T, path string, values url.Values) *http.Response {
	t.Helper()
	if values.Get("_csrf") == "" {
		values.Set("_csrf", e.csrf(t))
	}
	resp, err := e.client.PostForm(e.base+path, values)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) setup(t *testing.T) {
	t.Helper()

	resp := e.postForm(t, "/admin/setup", url.Values{
		"site_title": {"Test Blog"},
		"username":   {"admin"},
		"email":      {"admin@example.com"},
		"password":   {"password123"},
		"confirm":    {"password123"},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("setup status = %d body=%s", resp.StatusCode, body)
	}
}

func TestSetupCreatesOwnerAndLogsIn(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	user, err := e.store.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != model.RoleOwner {
		t.Fatalf("role = %q", user.Role)
	}
	if !auth.CheckPassword(user.PasswordHash, "password123") {
		t.Fatal("password was not hashed correctly")
	}

	resp := e.get(t, "/admin")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "概览") {
		t.Fatal("dashboard did not render")
	}

	// Setup is closed once an account exists.
	resp = e.get(t, "/admin/setup")
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("setup after install = %d, want 302", resp.StatusCode)
	}
}

func TestAdminRequiresLogin(t *testing.T) {
	e := newTestEnv(t)

	resp := e.get(t, "/admin/posts")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/admin/login") {
		t.Fatalf("location = %q", loc)
	}
}

func TestAdminRejectsMissingCSRF(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	// A fresh client has no CSRF cookie.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.PostForm(e.base+"/admin/posts", url.Values{"title": {"x"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestLoginAndLogout(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/logout", url.Values{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}

	resp = e.get(t, "/admin")
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("after logout status = %d", resp.StatusCode)
	}

	resp = e.postForm(t, "/admin/login", url.Values{
		"username": {"admin"},
		"password": {"wrong-password"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong login status = %d", resp.StatusCode)
	}

	resp = e.postForm(t, "/admin/login", url.Values{
		"username": {"admin"},
		"password": {"password123"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
}

func TestCreateEditAndDeletePost(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/posts", url.Values{
		"title":         {"第一篇"},
		"slug":          {"first"},
		"content":       {"# 标题\n\n正文"},
		"status":        {"publish"},
		"type":          {"post"},
		"math":          {"1"},
		"allow_comment": {"1"},
		"tags":          {"Go, 测试"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create status = %d", resp.StatusCode)
	}

	post, err := e.store.GetPost("first")
	if err != nil {
		t.Fatal(err)
	}
	if post.Title != "第一篇" || post.Status != model.StatusPublish || !post.Math {
		t.Fatalf("post = %+v", post)
	}
	if len(post.Tags) != 2 {
		t.Fatalf("tags = %+v", post.Tags)
	}

	resp = e.get(t, fmt.Sprintf("/admin/posts/%d/edit", post.ID))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "第一篇") {
		t.Fatalf("edit page status=%d", resp.StatusCode)
	}

	resp = e.postForm(t, fmt.Sprintf("/admin/posts/%d", post.ID), url.Values{
		"title":         {"改过的标题"},
		"content":       {"新正文"},
		"status":        {"draft"},
		"type":          {"post"},
		"math":          {"1"},
		"allow_comment": {"1"},
		"tags":          {"Go"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("update status = %d", resp.StatusCode)
	}

	updated, err := e.store.GetPost("first")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "改过的标题" || updated.Status != model.StatusDraft || len(updated.Tags) != 1 {
		t.Fatalf("updated = %+v", updated)
	}

	resp = e.postForm(t, fmt.Sprintf("/admin/posts/%d/delete", post.ID), url.Values{})
	resp.Body.Close()
	if _, err := e.store.GetPost("first"); err == nil {
		t.Fatal("post was not deleted")
	}
}

func TestAdminPreviewInlineMath(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/preview", url.Values{
		"content": {"Inline $x^2$."},
		"format":  {model.FormatMarkdown},
		"math":    {"1"},
	})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `\(x^2\)`) {
		t.Fatalf("preview status=%d body=%s", resp.StatusCode, body)
	}

	resp = e.postForm(t, "/admin/preview", url.Values{
		"content": {"PHP $x^2$."},
		"format":  {model.FormatMarkdown},
	})
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), `\(x^2\)`) {
		t.Fatalf("preview enabled math without opt-in: %s", body)
	}
}

type captureAdminEvents struct {
	events []events.Event
}

func (c *captureAdminEvents) Emit(_ context.Context, event events.Event) {
	c.events = append(c.events, event)
}

func TestAdminEmitsPostEvents(t *testing.T) {
	emitter := &captureAdminEvents{}
	e := newTestEnvWithEvents(t, emitter)
	e.setup(t)

	resp := e.postForm(t, "/admin/posts", url.Values{
		"title":   {"事件文章"},
		"slug":    {"event-post"},
		"content": {"正文"},
		"status":  {"publish"},
		"type":    {"post"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	if len(emitter.events) != 1 ||
		emitter.events[0].Type != events.PostCreated ||
		emitter.events[0].Source != events.SourceAdmin {
		t.Fatalf("events = %+v", emitter.events)
	}
	data, ok := emitter.events[0].Data.(events.PostData)
	if !ok || data.Post == nil || data.Post.Slug != "event-post" {
		t.Fatalf("data = %#v", emitter.events[0].Data)
	}
}

func TestCategoryAndTagManagement(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/categories/create", url.Values{
		"name": {"技术"}, "slug": {"tech"}, "sort_order": {"1"},
	})
	resp.Body.Close()

	c, err := e.store.GetCategory("tech")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "技术" {
		t.Fatalf("category = %+v", c)
	}

	resp = e.postForm(t, "/admin/categories/update", url.Values{
		"original_slug": {"tech"}, "name": {"技术文章"}, "slug": {"tech"}, "sort_order": {"2"},
	})
	resp.Body.Close()

	c, _ = e.store.GetCategory("tech")
	if c.Name != "技术文章" || c.SortOrder != 2 {
		t.Fatalf("category after update = %+v", c)
	}

	resp = e.postForm(t, "/admin/tags/create", url.Values{"name": {"随笔"}})
	resp.Body.Close()
	tag, err := e.store.GetTag("随笔")
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != "随笔" {
		t.Fatalf("tag = %+v", tag)
	}

	resp = e.postForm(t, "/admin/tags/delete", url.Values{"slug": {"随笔"}})
	resp.Body.Close()
	if _, err := e.store.GetTag("随笔"); err == nil {
		t.Fatal("tag was not deleted")
	}
}

func TestAdminHiddenTag(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/tags/create", url.Values{
		"name":     {"网安"},
		"slug":     {"security"},
		"hidden":   {"1"},
		"password": {"tag-pass"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create status = %d", resp.StatusCode)
	}

	tag, err := e.store.GetTag("security")
	if err != nil {
		t.Fatal(err)
	}
	if !tag.Hidden || !tag.HasPassword() {
		t.Fatalf("tag = %+v", tag)
	}
	if tag.PasswordHash == "tag-pass" || !auth.CheckPassword(tag.PasswordHash, "tag-pass") {
		t.Fatal("tag password was not hashed correctly")
	}

	resp = e.get(t, "/admin/tags")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "网安") || !strings.Contains(string(body), "已设置") {
		t.Fatalf("tags page status=%d body=%s", resp.StatusCode, body)
	}

	// Leaving the password empty keeps the existing hash.
	resp = e.postForm(t, "/admin/tags/update", url.Values{
		"original_slug": {"security"},
		"name":          {"网安"},
		"slug":          {"security"},
		"hidden":        {"1"},
	})
	resp.Body.Close()
	tag, _ = e.store.GetTag("security")
	if !tag.Hidden || !auth.CheckPassword(tag.PasswordHash, "tag-pass") {
		t.Fatalf("tag after keep = %+v", tag)
	}

	// A new password replaces the old one.
	resp = e.postForm(t, "/admin/tags/update", url.Values{
		"original_slug": {"security"},
		"name":          {"网安"},
		"slug":          {"security"},
		"hidden":        {"1"},
		"password":      {"new-tag-pass"},
	})
	resp.Body.Close()
	tag, _ = e.store.GetTag("security")
	if !auth.CheckPassword(tag.PasswordHash, "new-tag-pass") || auth.CheckPassword(tag.PasswordHash, "tag-pass") {
		t.Fatal("tag password was not replaced")
	}

	// Hidden tags cannot be created without a password.
	resp = e.postForm(t, "/admin/tags/create", url.Values{
		"name":   {"无密码"},
		"slug":   {"no-password"},
		"hidden": {"1"},
	})
	resp.Body.Close()
	if _, err := e.store.GetTag("no-password"); err == nil {
		t.Fatal("hidden tag without password was created")
	}
}

func TestAdminImportBundle(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.get(t, "/admin/import")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "导入 Typecho") {
		t.Fatalf("import page status=%d body=%s", resp.StatusCode, body)
	}

	var bundle bytes.Buffer
	zw := zip.NewWriter(&bundle)
	entry, _ := zw.Create("typecho.json")
	_, _ = entry.Write([]byte(`{"site":{"url":"https://example.com"},"contents":[],"metas":[],"comments":[],"users":[]}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	_ = mw.WriteField("_csrf", e.csrf(t))
	_ = mw.WriteField("dry_run", "1")
	part, _ := mw.CreateFormFile("bundle", "typecho-bundle.zip")
	_, _ = part.Write(bundle.Bytes())
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, e.base+"/admin/import", &form)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err = e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "导入报告") {
		t.Fatalf("import status=%d body=%s", resp.StatusCode, body)
	}
}

func TestAdminFriendLinks(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.get(t, "/admin/friends")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Akawa") {
		t.Fatalf("friends page status=%d body=%s", resp.StatusCode, body)
	}

	resp = e.postForm(t, "/admin/friends/create", url.Values{
		"name":        {"Test Friend"},
		"url":         {"https://friend.example/"},
		"avatar":      {"https://avatars.githubusercontent.com/u/1?v=4"},
		"description": {"friend.example"},
		"sort_order":  {"5"},
		"visible":     {"1"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	friends, err := e.store.ListFriendLinks(true)
	if err != nil {
		t.Fatal(err)
	}
	var created *model.FriendLink
	for _, f := range friends {
		if f.URL == "https://friend.example/" {
			created = f
		}
	}
	if created == nil || created.Name != "Test Friend" || !created.Visible {
		t.Fatalf("created = %+v", created)
	}

	resp = e.postForm(t, "/admin/friends/update", url.Values{
		"id":          {fmt.Sprint(created.ID)},
		"name":        {"Renamed Friend"},
		"url":         {created.URL},
		"avatar":      {created.Avatar},
		"description": {"renamed"},
		"sort_order":  {"8"},
	})
	resp.Body.Close()
	updated, err := e.store.GetFriendLink(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed Friend" || updated.Visible {
		t.Fatalf("updated = %+v", updated)
	}

	resp = e.postForm(t, "/admin/friends/delete", url.Values{"id": {fmt.Sprint(created.ID)}})
	resp.Body.Close()
	if _, err := e.store.GetFriendLink(created.ID); err == nil {
		t.Fatal("friend link was not deleted")
	}
}

func TestSettingsSave(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/settings", url.Values{
		"site.title":       {"新标题"},
		"site.description": {"描述"},
		"site.author":      {"作者"},
		"site.url":         {"https://example.com/"},
		"site.locale":      {"zh-CN"},
		"posts.per_page":   {"25"},
	})
	resp.Body.Close()

	if v, _ := e.store.GetOption("site.title"); v != "新标题" {
		t.Fatalf("site.title = %q", v)
	}
	if v, _ := e.store.GetOption("site.url"); v != "https://example.com" {
		t.Fatalf("site.url = %q", v)
	}
	if v, _ := e.store.GetOption("posts.per_page"); v != "25" {
		t.Fatalf("posts.per_page = %q", v)
	}
	if v, _ := e.store.GetOption("comments.enabled"); v != "0" {
		t.Fatalf("comments.enabled = %q", v)
	}
}

func TestProfilePasswordChange(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/profile", url.Values{
		"current_password": {"password123"},
		"password":         {"new-password-123"},
		"confirm":          {"new-password-123"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("profile status = %d", resp.StatusCode)
	}

	user, err := e.store.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(user.PasswordHash, "new-password-123") {
		t.Fatal("new password was not saved")
	}
	if auth.CheckPassword(user.PasswordHash, "password123") {
		t.Fatal("old password still works")
	}
}

func TestAdminUsers(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/users/create", url.Values{
		"username": {"writer"}, "email": {"writer@example.com"},
		"password": {"writer-pass"}, "role": {"editor"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create user status = %d", resp.StatusCode)
	}
	user, err := e.store.GetUserByUsername("writer")
	if err != nil {
		t.Fatal(err)
	}

	resp = e.get(t, "/admin/users")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "writer") {
		t.Fatalf("users page status = %d", resp.StatusCode)
	}

	resp = e.postForm(t, "/admin/users/delete", url.Values{"id": {fmt.Sprint(user.ID)}})
	resp.Body.Close()
	if _, err := e.store.GetUser(user.ID); err == nil {
		t.Fatal("user was not deleted")
	}
}

func TestEditorCannotOpenSettings(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	hash, err := auth.HashPassword("editor-pass")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.CreateUser(&model.User{
		Username: "editor", Email: "editor@example.com", PasswordHash: hash,
		Role: model.RoleEditor,
	}); err != nil {
		t.Fatal(err)
	}

	// Fresh client for the editor.
	jar, _ := cookiejar.New(nil)
	editor := &testEnv{server: e.server, client: &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, store: e.store, base: e.base, jar: jar}
	resp := editor.postForm(t, "/admin/login", url.Values{"username": {"editor"}, "password": {"editor-pass"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("editor login status = %d", resp.StatusCode)
	}

	resp = editor.get(t, "/admin/settings")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("editor settings status = %d, want 403", resp.StatusCode)
	}
}

func TestAdminNotes(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	resp := e.postForm(t, "/admin/notes/create", url.Values{"content": {"一条后台微语"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create note status = %d", resp.StatusCode)
	}

	notes, total, err := e.store.ListNotes(store.NoteFilter{})
	if err != nil || total != 1 {
		t.Fatalf("notes total=%d err=%v", total, err)
	}

	resp = e.get(t, "/admin/notes")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "一条后台微语") {
		t.Fatalf("notes page status=%d", resp.StatusCode)
	}

	resp = e.postForm(t, "/admin/notes/delete", url.Values{"id": {fmt.Sprint(notes[0].ID)}})
	resp.Body.Close()
	if _, total, _ := e.store.ListNotes(store.NoteFilter{}); total != 0 {
		t.Fatalf("note not deleted")
	}
}

func TestCommentModeration(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	post := &model.Post{Slug: "post", Title: "Post", Content: "body", Status: model.StatusPublish}
	if err := e.store.CreatePost(post); err != nil {
		t.Fatal(err)
	}
	comment := &model.Comment{PostID: post.ID, Author: "Reader", Content: "Nice"}
	if err := e.store.CreateComment(comment); err != nil {
		t.Fatal(err)
	}

	resp := e.postForm(t, "/admin/comments/status", url.Values{
		"id":     {fmt.Sprint(comment.ID)},
		"action": {model.CommentApproved},
		"filter": {"pending"},
		"page":   {"1"},
	})
	resp.Body.Close()

	got, err := e.store.GetComment(comment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.CommentApproved {
		t.Fatalf("status = %q", got.Status)
	}

	resp = e.postForm(t, "/admin/comments/delete", url.Values{
		"id": {fmt.Sprint(comment.ID)}, "filter": {"approved"}, "page": {"1"},
	})
	resp.Body.Close()
	if _, err := e.store.GetComment(comment.ID); err == nil {
		t.Fatal("comment was not deleted")
	}
}

func TestMediaUpload(t *testing.T) {
	e := newTestEnv(t)
	e.setup(t)

	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("_csrf", e.csrf(t))
	part, err := writer.CreateFormFile("file", "pixel.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(img.Bytes())
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, e.base+"/admin/media/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

	items, total, err := e.store.ListMedia(1, 10)
	if err != nil || total != 1 {
		t.Fatalf("media total=%d err=%v", total, err)
	}
	if items[0].MIME != "image/png" || items[0].Width != 1 || items[0].Height != 1 {
		t.Fatalf("media = %+v", items[0])
	}

	// HTML uploads must be rejected.
	body.Reset()
	writer = multipart.NewWriter(body)
	writer.WriteField("_csrf", e.csrf(t))
	part, _ = writer.CreateFormFile("file", "evil.html")
	part.Write([]byte("<html><script>alert(1)</script></html>"))
	writer.Close()

	req, _ = http.NewRequest(http.MethodPost, e.base+"/admin/media/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err = e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("html upload status = %d, want 400", resp.StatusCode)
	}
}
