package content

import (
	"strings"
	"testing"

	"github.com/pionus/pionus-go/internal/model"
)

func TestRenderMarkdownPost(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "## 第一节\n\n正文 $$x^2$$\n\n```go\nfmt.Println(\"hi\")\n```\n\n### 子节\n",
	}
	got := RenderPost(p)

	if !strings.Contains(string(got.HTML), "<h2") || !strings.Contains(string(got.HTML), "第一节") {
		t.Fatalf("html = %s", got.HTML)
	}
	if len(got.TOC) != 2 {
		t.Fatalf("toc = %+v", got.TOC)
	}
	if got.TOC[0].Text != "第一节" || got.TOC[0].Level != 2 || got.TOC[0].ID == "" {
		t.Fatalf("toc[0] = %+v", got.TOC[0])
	}
	if !got.HasMath {
		t.Fatal("math was not detected")
	}
	if !got.HasCode {
		t.Fatal("code was not detected")
	}
	if !strings.Contains(string(got.HTML), `class="language-go"`) {
		t.Fatalf("code fence lost language: %s", got.HTML)
	}
}

func TestRenderLegacyHTML(t *testing.T) {
	p := &model.Post{
		Format: model.FormatHTML,
		Content: `<h2>旧标题</h2>
<p style="font-family:Arial;color:#cc3300;width:900px;line-height:1.2">正文<wbr><span></span></p>
<pre class="brush:js; toolbar: true;">var x = 1;</pre>
<img src="http://old.example/a.png" width="800" height="600">
<table><tr><td>cell</td></tr></table>
<a href="https://example.com">link</a>`,
	}
	got := RenderPost(p)

	html := string(got.HTML)
	if strings.Contains(html, "<wbr>") || strings.Contains(html, "<span></span>") {
		t.Fatalf("legacy artifacts remain: %s", html)
	}
	if strings.Contains(html, "font-family") || strings.Contains(html, "line-height") || strings.Contains(html, `width="800"`) {
		t.Fatalf("layout styles remain: %s", html)
	}
	if !strings.Contains(html, "color:#cc3300") {
		t.Fatalf("emphasis color should be kept: %s", html)
	}
	if !strings.Contains(html, `class="language-js"`) {
		t.Fatalf("brush class was not converted: %s", html)
	}
	if !strings.Contains(html, `loading="lazy"`) || !strings.Contains(html, `src="https://old.example/a.png"`) {
		t.Fatalf("image was not normalized: %s", html)
	}
	if !strings.Contains(html, `class="table-wrap"`) {
		t.Fatalf("table was not wrapped: %s", html)
	}
	if !strings.Contains(html, `rel="noopener noreferrer"`) {
		t.Fatalf("external link rel missing: %s", html)
	}
	if len(got.TOC) != 1 || got.TOC[0].Text != "旧标题" || got.TOC[0].ID == "" {
		t.Fatalf("toc = %+v", got.TOC)
	}
}

func TestInlineMathRequiresOptIn(t *testing.T) {
	php := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "PHP uses $name and $_REQUEST.",
	}
	if got := RenderPost(php); got.HasMath || got.InlineMath {
		t.Fatalf("PHP dollars enabled math: %+v", got)
	}

	math := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "Inline $x^2$ and cost $5.",
		Math:    true,
	}
	got := RenderPost(math)
	if !got.HasMath || !got.InlineMath {
		t.Fatalf("opted-in inline math not detected: %+v", got)
	}
	if !strings.Contains(string(got.HTML), `\(x^2\)`) || strings.Contains(string(got.HTML), "$x^2$") {
		t.Fatalf("inline math was not converted: %s", got.HTML)
	}

	display := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "Display $$x^2$$ only.",
	}
	got = RenderPost(display)
	if !got.HasMath || got.InlineMath {
		t.Fatalf("display math should not enable inline delimiters: %+v", got)
	}
}

func TestInlineMathIgnoresEscapedAndDoubleDollars(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatMarkdown,
		Content: `Escaped \$5 and a lone $sign.`,
		Math:    true,
	}
	if got := RenderPost(p); got.HasMath {
		t.Fatal("escaped/unpaired dollars should not count as inline math")
	}
}

func TestInlineMathProtectsMarkdownSyntax(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "Price $5 and $10. Formula $a_i + b^2$.",
		Math:    true,
	}
	got := RenderPost(p)
	html := string(got.HTML)
	if !strings.Contains(html, "Price $5 and $10") {
		t.Fatalf("currency was consumed: %s", html)
	}
	if !strings.Contains(html, `\(a_i + b^2\)`) {
		t.Fatalf("formula was not converted: %s", html)
	}
	if strings.Contains(html, "<em>") {
		t.Fatalf("formula was parsed as Markdown emphasis: %s", html)
	}
}

func TestInlineMathWithoutFormulaDoesNotLoadKatex(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "Price $5 and $10.",
		Math:    true,
	}
	got := RenderPost(p)
	if got.HasMath || got.InlineMath {
		t.Fatalf("currency enabled math: %+v", got)
	}
	if !strings.Contains(string(got.HTML), "$5 and $10") {
		t.Fatalf("currency was changed: %s", got.HTML)
	}
}

func TestInlineMathInHTMLContent(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatHTML,
		Content: "<p>Formula $x^2$ and code <code>$not$</code>.</p>",
		Math:    true,
	}
	got := RenderPost(p)
	html := string(got.HTML)
	if !strings.Contains(html, `\(x^2\)`) {
		t.Fatalf("HTML inline math was not converted: %s", html)
	}
	if !strings.Contains(html, "<code>$not$</code>") {
		t.Fatalf("code span was modified: %s", html)
	}
}

func TestInlineMathInMarkdownRawHTML(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatMarkdown,
		Content: "<div>Formula $x^2$</div>",
		Math:    true,
	}
	got := RenderPost(p)
	if !strings.Contains(string(got.HTML), `\(x^2\)`) {
		t.Fatalf("raw HTML inline math was not converted: %s", got.HTML)
	}
}

func TestRenderHTMLHeadingIDsUnique(t *testing.T) {
	p := &model.Post{
		Format:  model.FormatHTML,
		Content: "<h2>重复</h2><h2>重复</h2><h3>重复</h3>",
	}
	got := RenderPost(p)
	if len(got.TOC) != 3 {
		t.Fatalf("toc = %+v", got.TOC)
	}
	seen := map[string]bool{}
	for _, item := range got.TOC {
		if seen[item.ID] {
			t.Fatalf("duplicate id %q in %+v", item.ID, got.TOC)
		}
		seen[item.ID] = true
	}
}
