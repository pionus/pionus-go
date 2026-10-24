package imageaudit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func TestAudit(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(filepath.Join(uploads, "2024", "01"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "2024", "01", "ok.png"), []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	posts := []*model.Post{
		{
			Slug: "a", Title: "A", Status: model.StatusPublish,
			Content: `<img src="/uploads/2024/01/ok.png"><img src="/uploads/2024/01/missing.png"><img src="http://old.example/x.jpg">`,
		},
		{
			Slug: "b", Title: "B", Status: model.StatusPublish,
			Content: `![pic](/uploads/2024/01/ok.png) and ![gone](/uploads/nope.png)`,
		},
	}
	for _, p := range posts {
		if err := db.CreatePost(p); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Audit(db, uploads, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Posts != 2 {
		t.Fatalf("posts = %d", report.Posts)
	}
	if report.Local != 4 || report.Missing != 2 {
		t.Fatalf("local=%d missing=%d", report.Local, report.Missing)
	}
	if report.External != 1 || report.Hosts["old.example"] != 1 {
		t.Fatalf("external=%d hosts=%v", report.External, report.Hosts)
	}
	if len(report.Items) != 2 {
		t.Fatalf("items = %+v", report.Items)
	}
}
