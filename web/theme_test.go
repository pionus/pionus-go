package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pionus/pionus-go/internal/content"
	"github.com/pionus/pionus-go/internal/model"
)

func siteData(title string) map[string]any {
	return map[string]any{
		"Site": map[string]any{
			"Title":       "Secbone's Blog",
			"Description": "Just So So ...",
			"Author":      "Secbone",
			"URL":         "https://example.com",
			"Keywords":    "go, blog",
			"Locale":      "zh-CN",
		},
		"Title":        title,
		"CurrentPath":  "/article",
		"Year":         2026,
		"SearchQuery":  "",
		"NavPages":     []*model.Post{},
		"CanonicalURL": "https://example.com/article",
	}
}

func samplePost() *model.Post {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return &model.Post{
		ID:           1,
		Slug:         "hello",
		Title:        "Hello World",
		Content:      "## Hi\n\nbody",
		Excerpt:      "body",
		Type:         model.PostTypePost,
		Status:       model.StatusPublish,
		Format:       model.FormatMarkdown,
		Author:       "Secbone",
		AllowComment: true,
		Views:        12,
		PublishedAt:  &at,
		CreatedAt:    at,
		UpdatedAt:    at,
		Categories:   []*model.Category{{Name: "Tech", Slug: "tech"}},
		Tags:         []*model.Tag{{Name: "Go", Slug: "go"}},
	}
}

func renderTemplate(t *testing.T, name string, data map[string]any) string {
	t.Helper()
	engine := NewEngine()
	var buf bytes.Buffer
	if err := engine.Render(&buf, name, data, nil); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Fatalf("%s rendered empty output", name)
	}
	return buf.String()
}

func TestHomeTemplate(t *testing.T) {
	data := siteData("Secbone's Blog")
	data["Posts"] = []*model.Post{samplePost()}
	data["Categories"] = []*model.Category{{Name: "Tech", Slug: "tech", PostCount: 3}}
	data["Tags"] = []*model.Tag{{Name: "Go", Slug: "go"}}
	out := renderTemplate(t, "home.html", data)
	if !strings.Contains(out, "Hello World") || !strings.Contains(out, "最近文章") {
		t.Fatalf("home output missing content")
	}
}

func TestArchiveTemplate(t *testing.T) {
	data := siteData("文章归档")
	data["Groups"] = []map[string]any{
		{"Year": 2026, "Posts": []*model.Post{samplePost()}},
	}
	data["Total"] = 1
	data["TotalPages"] = 1
	data["Page"] = 1
	data["HasPrev"] = false
	data["HasNext"] = false
	data["PrevURL"] = ""
	data["NextURL"] = ""
	data["ArchiveTitle"] = "全部文章"
	data["ArchiveKind"] = "all"
	out := renderTemplate(t, "archive.html", data)
	if !strings.Contains(out, "2026") || !strings.Contains(out, "Hello World") {
		t.Fatalf("archive output missing content")
	}
}

func postTemplateData(post *model.Post) map[string]any {
	rendered := content.RenderPost(post)
	data := siteData(post.Title)
	data["Post"] = post
	data["Rendered"] = rendered
	data["TOC"] = rendered.TOC
	data["ReadingTime"] = 3
	data["Prev"] = nil
	data["Next"] = nil
	data["Comments"] = []any{}
	data["CommentCount"] = 0
	data["CanComment"] = true
	data["CSRF"] = "token"
	data["CommentAction"] = "/article/hello/comment"
	data["CommentError"] = ""
	data["CommentMessage"] = ""
	data["Form"] = struct {
		Author  string
		Email   string
		URL     string
		Content string
		Parent  int64
		Started int64
	}{Started: time.Now().Unix()}
	data["IsPage"] = false
	return data
}

func TestPostTemplate(t *testing.T) {
	data := postTemplateData(samplePost())
	out := renderTemplate(t, "post.html", data)
	for _, want := range []string{"Hello World", "本文目录", "写下你的评论", `name="_csrf"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("post output missing %q", want)
		}
	}
}

func TestPostTemplateInlineMath(t *testing.T) {
	post := samplePost()
	post.Math = true
	post.Content = "Inline $x^2$."
	out := renderTemplate(t, "post.html", postTemplateData(post))
	if !strings.Contains(out, "katex.min.js") {
		t.Fatal("KaTeX was not loaded for inline math")
	}
	if strings.Contains(out, "{left: '$', right: '$'") {
		t.Fatal("client-side dollar delimiter should not be enabled")
	}
	if !strings.Contains(out, `\(x^2\)`) {
		t.Fatal("inline math was not converted to an explicit delimiter")
	}
}

func TestTagPasswordTemplate(t *testing.T) {
	data := siteData("网安")
	data["Tag"] = &model.Tag{Name: "网安", Slug: "security", Hidden: true}
	data["UnlockAction"] = "/tag/security/unlock"
	data["Error"] = "密码不正确。"
	data["CSRF"] = "token"
	out := renderTemplate(t, "tag-password.html", data)
	for _, want := range []string{"网安", "这个标签下的文章需要密码", "密码不正确", `name="_csrf"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("tag password output missing %q", want)
		}
	}
}

func TestNotesTemplate(t *testing.T) {
	data := siteData("微语")
	data["Notes"] = []map[string]any{
		{"ID": 7, "HTML": template.HTML("<p>一条微语</p>"), "CreatedAt": time.Now()},
	}
	data["Total"] = 1
	data["Page"] = 1
	data["TotalPages"] = 1
	data["HasPrev"] = false
	data["HasNext"] = false
	data["PrevURL"] = ""
	data["NextURL"] = ""

	out := renderTemplate(t, "notes.html", data)
	if !strings.Contains(out, "一条微语") || !strings.Contains(out, "timeline-item") || !strings.Contains(out, `id="note-7"`) {
		t.Fatalf("notes output missing content")
	}
}

func TestTaxonomyAnd404Templates(t *testing.T) {
	data := siteData("分类")
	data["Categories"] = []*model.Category{{Name: "Tech", Slug: "tech", PostCount: 3}}
	renderTemplate(t, "categories.html", data)

	data = siteData("标签")
	data["Tags"] = []*model.Tag{{Name: "Go", Slug: "go", PostCount: 2}}
	renderTemplate(t, "tags.html", data)

	data = siteData("页面没找到")
	data["Message"] = "页面不存在"
	renderTemplate(t, "404.html", data)
}

func TestExternalThemeOverlaysEmbedded(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pages"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pages", "home.html"), []byte(`{{define "home.html"}}CUSTOM HOME{{end}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "custom.css"), []byte("body{color:red}"), 0644); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngineWithTheme(dir)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := engine.Render(&buf, "home.html", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "CUSTOM HOME" {
		t.Fatalf("home = %q", got)
	}

	templates, assets, err := ThemeFilesystems(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(templates, "pages/post.html"); err != nil {
		t.Fatalf("embedded template fallback missing: %v", err)
	}
	data, err := fs.ReadFile(assets, "custom.css")
	if err != nil || string(data) != "body{color:red}" {
		t.Fatalf("custom asset = %q, err=%v", data, err)
	}
	if _, err := fs.Stat(assets, "main.css"); err != nil {
		t.Fatalf("embedded asset fallback missing: %v", err)
	}

	if _, _, err := ThemeFilesystems(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing theme directory error")
	}
}
