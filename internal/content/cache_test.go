package content

import (
	"strings"
	"testing"

	"github.com/pionus/pionus-go/internal/model"
)

func TestRenderPostCachedInvalidatesOnContentChange(t *testing.T) {
	p := &model.Post{ID: 42, Format: model.FormatMarkdown, Content: "## First"}
	first := RenderPostCached(p)
	if !strings.Contains(string(first.HTML), "First") {
		t.Fatalf("first render = %s", first.HTML)
	}

	p.Content = "## Second"
	second := RenderPostCached(p)
	if !strings.Contains(string(second.HTML), "Second") || strings.Contains(string(second.HTML), "First") {
		t.Fatalf("cache did not invalidate: %s", second.HTML)
	}
}
