package content

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var noteMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
)

// RenderNote renders a short post for the 微语 timeline. Hard wraps keep the
// line breaks from imported plain-text notes.
func RenderNote(source string) template.HTML {
	var buf bytes.Buffer
	if err := noteMarkdown.Convert([]byte(source), &buf); err != nil {
		return ""
	}
	return template.HTML(buf.String())
}
