package typecho

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func sampleExport() Export {
	return Export{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Site: Site{
			URL:         "https://old.example",
			Title:       "Old Blog",
			Description: "old description",
			Timezone:    8 * 3600,
			PageSize:    10,
		},
		Users: []User{{
			UID:        1,
			Name:       "Secbone",
			ScreenName: "Secbone",
			Mail:       "me@example.com",
			URL:        "https://old.example",
			Group:      "administrator",
		}},
		Metas: []Meta{
			{MID: 1, Name: "技术", Slug: "tech", Type: "category", Order: 1},
			{MID: 2, Name: "Go", Slug: "go", Type: "tag"},
		},
		Relationships: []Relationship{
			{CID: 10, MID: 1},
			{CID: 10, MID: 2},
			{CID: 11, MID: 1},
		},
		Contents: []Content{
			{
				CID: 10, Title: "HTML 文章", Created: 1325116801,
				Text: "<p>Hello <b>HTML</b></p><img src=\"/usr/uploads/2015/03/a.jpg\"><img src=\"https://old.example/usr/uploads/2015/03/a.jpg\"><a href=\"http://old.example/index.php/archives/11/\">next</a><a href=\"/index.php/about.html\">about</a><a href=\"http://old.example/index.php/category/tech/\">cat</a>", Type: "post", Status: "publish",
				AllowComment: "1", AuthorID: 1, Views: 7,
			},
			{
				CID: 11, Title: "Markdown 文章", Slug: "md-post", Created: 1503496680,
				Text: "<!--markdown--># Hi\n\n**bold**", Type: "post", Status: "publish",
				AllowComment: "0", AuthorID: 1,
			},
			{
				CID: 12, Title: "关于", Slug: "about", Created: 1400000000,
				Text: "<!--markdown-->about page", Type: "page", Status: "publish",
				AllowComment: "1", AuthorID: 1, Template: "about.php",
			},
			{
				CID: 13, Title: "a.jpg", Type: "attachment", Status: "publish",
				Text: `a:5:{s:4:"name";s:5:"a.jpg";s:4:"path";s:31:"/usr/uploads/2015/03/a.jpg";s:4:"size";i:12;s:4:"type";s:3:"jpg";s:4:"mime";s:10:"image/jpeg";}`,
			},
		},
		Comments: []Comment{
			{Coid: 1, CID: 10, Author: "A", Mail: "a@example.com", Text: "first", Status: "approved", Created: 1326000000},
			{Coid: 2, CID: 10, Parent: 1, Author: "B", Text: "reply <img src=\"/usr/uploads/2015/03/a.jpg\"> <a href=\"http://old.example/index.php/archives/10/\">self</a>", Status: "waiting", Created: 1326000100},
		},
	}
}

func newStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func exportJSON(t *testing.T, ex Export) []byte {
	t.Helper()
	data, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestImport(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("CST", 8*3600)
	t.Cleanup(func() { time.Local = oldLocal })

	db := newStore(t)
	rep, err := Import(bytes.NewReader(exportJSON(t, sampleExport())), Options{Store: db})
	if err != nil {
		t.Fatal(err)
	}

	if rep.Users != 1 || rep.Posts != 2 || rep.Pages != 1 || rep.Comments != 2 {
		t.Fatalf("report = %+v", rep)
	}

	user, err := db.GetUserByUsername("Secbone")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != model.RoleOwner || user.PasswordHash != "" {
		t.Fatalf("user = %+v", user)
	}

	htmlPost, err := db.GetPost("archives-10")
	if err != nil {
		t.Fatal(err)
	}
	if htmlPost.Format != model.FormatHTML {
		t.Fatalf("format = %q", htmlPost.Format)
	}
	if htmlPost.Source != Source || htmlPost.SourceID != "10" {
		t.Fatalf("source = %q/%q", htmlPost.Source, htmlPost.SourceID)
	}
	if htmlPost.Views != 7 || htmlPost.Author != "Secbone" {
		t.Fatalf("post = %+v", htmlPost)
	}
	if strings.Contains(htmlPost.Content, "/usr/uploads/") || strings.Count(htmlPost.Content, "/uploads/2015/03/a.jpg") != 2 {
		t.Fatalf("asset paths were not rewritten: %q", htmlPost.Content)
	}
	for _, want := range []string{"/article/md-post", "/article/about", "/category/tech"} {
		if !strings.Contains(htmlPost.Content, want) {
			t.Fatalf("internal link %q was not rewritten: %q", want, htmlPost.Content)
		}
	}
	if len(htmlPost.Categories) != 1 || htmlPost.Categories[0].Slug != "tech" {
		t.Fatalf("categories = %+v", htmlPost.Categories)
	}
	if len(htmlPost.Tags) != 1 || htmlPost.Tags[0].Slug != "go" {
		t.Fatalf("tags = %+v", htmlPost.Tags)
	}
	wantTime := time.Unix(1325116801, 0).In(time.Local)
	if htmlPost.CreatedAt.Format("2006-01-02 15:04") != wantTime.Format("2006-01-02 15:04") {
		t.Fatalf("created = %s, want %s", htmlPost.CreatedAt, wantTime)
	}

	mdPost, err := db.GetPost("md-post")
	if err != nil {
		t.Fatal(err)
	}
	if mdPost.Format != model.FormatMarkdown || mdPost.AllowComment {
		t.Fatalf("markdown post = %+v", mdPost)
	}
	if !strings.HasPrefix(mdPost.Content, "# Hi") {
		t.Fatalf("marker was not stripped: %q", mdPost.Content)
	}

	page, err := db.GetPost("about")
	if err != nil {
		t.Fatal(err)
	}
	if page.Type != model.PostTypePage || page.Template != "about.php" {
		t.Fatalf("page = %+v", page)
	}

	comments, total, err := db.ListComments(store.CommentFilter{PostID: htmlPost.ID})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(comments) != 2 {
		t.Fatalf("comments total=%d len=%d", total, len(comments))
	}
	var reply *model.Comment
	for _, c := range comments {
		if c.Author == "B" {
			reply = c
		}
	}
	if reply == nil || reply.ParentID == nil || reply.Status != model.CommentPending {
		t.Fatalf("reply = %+v", reply)
	}
	if strings.Contains(reply.Content, "/usr/uploads/") {
		t.Fatalf("comment asset path not rewritten: %q", reply.Content)
	}
	if !strings.Contains(reply.Content, "/article/archives-10") {
		t.Fatalf("comment post link not rewritten: %q", reply.Content)
	}

	if v, _ := db.GetOption("site.title"); v != "Old Blog" {
		t.Fatalf("site.title = %q", v)
	}
	if v, _ := db.GetOption("site.author"); v != "Secbone" {
		t.Fatalf("site.author = %q", v)
	}

	if len(rep.MissingUploads) == 0 {
		t.Fatal("expected missing attachment to be reported")
	}
}

func TestImportIsIdempotent(t *testing.T) {
	db := newStore(t)
	data := exportJSON(t, sampleExport())

	if _, err := Import(bytes.NewReader(data), Options{Store: db}); err != nil {
		t.Fatal(err)
	}
	rep, err := Import(bytes.NewReader(data), Options{Store: db})
	if err != nil {
		t.Fatal(err)
	}

	_, total, err := db.ListPosts(store.PostFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("posts after re-import = %d, want 3", total)
	}
	if rep.Updated != 3 {
		t.Fatalf("updated = %d, want 3", rep.Updated)
	}

	_, comments, err := db.ListComments(store.CommentFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if comments != 2 {
		t.Fatalf("comments after re-import = %d, want 2", comments)
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	db := newStore(t)
	rep, err := Import(bytes.NewReader(exportJSON(t, sampleExport())), Options{Store: db, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Posts != 2 || rep.Pages != 1 {
		t.Fatalf("report = %+v", rep)
	}

	if users, _ := db.ListUsers(); len(users) != 0 {
		t.Fatalf("dry run created %d users", len(users))
	}
	if _, total, _ := db.ListPosts(store.PostFilter{}); total != 0 {
		t.Fatalf("dry run created %d posts", total)
	}
}

func TestImportAttachmentCopy(t *testing.T) {
	db := newStore(t)
	src := t.TempDir()
	dst := t.TempDir()

	oldPath := filepath.Join(src, "2015", "03", "a.jpg")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("jpeg-bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	rep, err := Import(bytes.NewReader(exportJSON(t, sampleExport())), Options{
		Store:         db,
		SourceUploads: src,
		DestUploads:   dst,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Attachments != 1 || len(rep.MissingUploads) != 0 {
		t.Fatalf("report = %+v", rep)
	}

	if _, err := os.Stat(filepath.Join(dst, "2015", "03", "a.jpg")); err != nil {
		t.Fatalf("attachment was not copied: %v", err)
	}
	media, total, err := db.ListMedia(1, 10)
	if err != nil || total != 1 {
		t.Fatalf("media total=%d err=%v", total, err)
	}
	if media[0].Path != "2015/03/a.jpg" || media[0].MIME != "image/jpeg" {
		t.Fatalf("media = %+v", media[0])
	}
}

func TestParseContentText(t *testing.T) {
	format, body := parseContentText("\n<!--markdown--># Hi")
	if format != model.FormatMarkdown || body != "# Hi" {
		t.Fatalf("format=%q body=%q", format, body)
	}
	format, body = parseContentText("<p>hi</p>")
	if format != model.FormatHTML || body != "<p>hi</p>" {
		t.Fatalf("format=%q body=%q", format, body)
	}
}
