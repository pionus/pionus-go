package content

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// RenderMarkdown converts markdown to HTML. Raw HTML is allowed because only
// signed-in authors can write posts.
func RenderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(source), &buf); err != nil {
		return ""
	}
	return template.HTML(buf.String())
}
