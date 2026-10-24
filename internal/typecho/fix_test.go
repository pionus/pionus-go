package typecho

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func TestFixLinks(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SetOption("site.url", "https://old.example"); err != nil {
		t.Fatal(err)
	}

	post := &model.Post{
		Slug: "archives-10", Title: "Post", Status: model.StatusPublish,
		Content: `see <a href="http://old.example/index.php/archives/10/">self</a> ` +
			`<a href="/index.php/archives/10/">self</a> ` +
			`<a href="/index.php/about.html">about</a> ` +
			`<a href="/index.php/category/tech/">tech</a> ` +
			`<img src="/usr/uploads/2015/03/a.jpg">`,
		Source: Source, SourceID: "10",
	}
	if err := db.CreatePost(post); err != nil {
		t.Fatal(err)
	}
	page := &model.Post{
		Slug: "about", Title: "About", Type: model.PostTypePage,
		Status: model.StatusPublish, Content: "about",
		Source: Source, SourceID: "11",
	}
	if err := db.CreatePost(page); err != nil {
		t.Fatal(err)
	}
	comment := &model.Comment{
		PostID: post.ID, Author: "A", Content: "link /index.php/archives/10/",
	}
	if err := db.CreateComment(comment); err != nil {
		t.Fatal(err)
	}

	report, err := FixLinks(db, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.PostsUpdated != 1 || report.CommentsUpdated != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.RemainingOldLinks) != 0 {
		t.Fatalf("remaining = %v", report.RemainingOldLinks)
	}

	got, err := db.GetPost("archives-10")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/article/archives-10", "/article/about", "/category/tech", "/uploads/2015/03/a.jpg"} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("missing %q in %q", want, got.Content)
		}
	}
	for _, bad := range []string{"/index.php/", "/usr/uploads/", "old.example"} {
		if strings.Contains(got.Content, bad) {
			t.Fatalf("old content %q remains in %q", bad, got.Content)
		}
	}

	gotComment, err := db.GetComment(comment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotComment.Content, "/index.php/") || !strings.Contains(gotComment.Content, "/article/archives-10") {
		t.Fatalf("comment = %q", gotComment.Content)
	}

	// Second pass is a no-op.
	again, err := FixLinks(db, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.PostsUpdated != 0 || again.CommentsUpdated != 0 {
		t.Fatalf("second pass = %+v", again)
	}
}

func TestFixLinksDryRun(t *testing.T) {
	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SetOption("site.url", "https://old.example"); err != nil {
		t.Fatal(err)
	}
	post := &model.Post{
		Slug: "p", Title: "P", Status: model.StatusPublish,
		Content: "http://old.example/index.php/archives/10/",
		Source:  Source, SourceID: "10",
	}
	if err := db.CreatePost(post); err != nil {
		t.Fatal(err)
	}

	report, err := FixLinks(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.PostsUpdated != 1 {
		t.Fatalf("report = %+v", report)
	}
	got, _ := db.GetPost("p")
	if !strings.Contains(got.Content, "/index.php/") {
		t.Fatal("dry run modified the post")
	}
}
