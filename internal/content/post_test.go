package content

import (
	"errors"
	"testing"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

func TestBuildRequiresTitle(t *testing.T) {
	_, err := (&PostInput{}).Build(nil)
	if !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildCreateDefaults(t *testing.T) {
	p, err := (&PostInput{Title: "Hello World", Content: "body"}).Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "hello-world" {
		t.Fatalf("slug = %q", p.Slug)
	}
	if p.Status != model.StatusPublish || p.Type != model.PostTypePost {
		t.Fatalf("status/type = %s/%s", p.Status, p.Type)
	}
	if !p.AllowComment {
		t.Fatal("comments should be allowed by default")
	}
	if p.Excerpt == "" {
		t.Fatal("excerpt was not generated")
	}
}

func TestBuildValidatesSlug(t *testing.T) {
	_, err := (&PostInput{Title: "T", Content: "C", Slug: "bad/slug"}).Build(nil)
	if err == nil {
		t.Fatal("expected slug validation error")
	}
}

func TestBuildValidatesTypeAndStatus(t *testing.T) {
	if _, err := (&PostInput{Title: "T", Content: "C", Type: "nope"}).Build(nil); err == nil {
		t.Fatal("expected type error")
	}
	if _, err := (&PostInput{Title: "T", Content: "C", Status: "nope"}).Build(nil); err == nil {
		t.Fatal("expected status error")
	}
}

func TestBuildUpdateKeepsFields(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	existing := &model.Post{
		ID:           7,
		Slug:         "keep",
		Title:        "Old",
		Content:      "old body",
		Type:         model.PostTypePost,
		Status:       model.StatusPublish,
		AllowComment: true,
		Pinned:       true,
		Views:        42,
		PublishedAt:  &at,
		CreatedAt:    at.Add(-time.Hour),
	}

	p, err := (&PostInput{Title: "New", Content: "new body"}).Build(existing)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 7 || p.Slug != "keep" || p.Title != "New" {
		t.Fatalf("post = %+v", p)
	}
	if !p.Pinned || !p.AllowComment || p.Views != 42 {
		t.Fatalf("preserved fields lost: %+v", p)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(at) {
		t.Fatalf("published_at = %v", p.PublishedAt)
	}
}

func TestBuildUpdateCanClearTaxonomy(t *testing.T) {
	existing := &model.Post{Slug: "p", Title: "P", Content: "c", Status: model.StatusDraft}

	p, err := (&PostInput{Title: "P", Content: "c", CategoryIDs: []int64{}, TagIDs: []int64{}}).Build(existing)
	if err != nil {
		t.Fatal(err)
	}
	if p.CategoryIDs == nil || len(p.CategoryIDs) != 0 {
		t.Fatalf("category ids = %#v", p.CategoryIDs)
	}
	if p.TagIDs == nil || len(p.TagIDs) != 0 {
		t.Fatalf("tag ids = %#v", p.TagIDs)
	}
}
