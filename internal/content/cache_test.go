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

func TestRenderPostCachedIncludesMathFlag(t *testing.T) {
	p := &model.Post{ID: 43, Format: model.FormatMarkdown, Content: "Inline $x$"}
	if first := RenderPostCached(p); first.InlineMath {
		t.Fatal("inline math enabled before opt-in")
	}

	p.Math = true
	second := RenderPostCached(p)
	if !second.InlineMath || !second.HasMath {
		t.Fatalf("math flag did not invalidate cache: %+v", second)
	}
}
