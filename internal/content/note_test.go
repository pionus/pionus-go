package content

import (
	"strings"
	"testing"
)

func TestRenderNote(t *testing.T) {
	out := string(RenderNote("第一行\n第二行\n\n**加粗**"))
	if !strings.Contains(out, "<br") {
		t.Fatalf("hard wraps missing: %s", out)
	}
	if !strings.Contains(out, "<strong>加粗</strong>") {
		t.Fatalf("markdown missing: %s", out)
	}
}
