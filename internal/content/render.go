package content

import (
	"bytes"
	"html/template"
	"strconv"
	"strings"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	golanghtml "golang.org/x/net/html"
)

// TOCItem is one heading in the table of contents.
type TOCItem struct {
	ID    string
	Text  string
	Level int
}

// RenderedPost is the processed content of a post.
type RenderedPost struct {
	HTML    template.HTML
	TOC     []TOCItem
	HasMath bool
	HasCode bool
}

var postMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// RenderPost converts a post body to display HTML, normalizes legacy markup,
// assigns heading IDs and extracts a table of contents.
func RenderPost(p *model.Post) RenderedPost {
	var raw string
	if p.Format == model.FormatHTML {
		raw = p.Content
	} else {
		var buf bytes.Buffer
		if err := postMarkdown.Convert([]byte(p.Content), &buf); err != nil {
			raw = p.Content
		} else {
			raw = buf.String()
		}
	}

	normalized, toc := normalizeContent(raw)
	return RenderedPost{
		HTML:    template.HTML(normalized),
		TOC:     toc,
		HasMath: hasMath(p.Content),
		HasCode: strings.Contains(normalized, "<pre"),
	}
}

// hasMath avoids enabling KaTeX for ordinary dollar signs in PHP or shell
// snippets. Display math and explicit LaTeX delimiters are unambiguous.
func hasMath(source string) bool {
	return strings.Contains(source, "$$") ||
		strings.Contains(source, "\\(") ||
		strings.Contains(source, "\\[")
}

// normalizeContent cleans legacy Typecho HTML and builds a TOC.
func normalizeContent(source string) (string, []TOCItem) {
	doc, err := golanghtml.Parse(strings.NewReader(source))
	if err != nil {
		return source, nil
	}

	body := findNode(doc, "body")
	if body == nil {
		return source, nil
	}

	ids := make(map[string]int)
	var toc []TOCItem
	walk(body, func(n *golanghtml.Node) {
		if n.Type != golanghtml.ElementNode {
			return
		}
		n.Attr = dropSizeAttrs(n.Attr)

		switch n.Data {
		case "h1", "h2", "h3", "h4":
			level := int(n.Data[1] - '0')
			text := strings.TrimSpace(nodeText(n))
			if text == "" {
				return
			}
			id := attr(n, "id")
			if id == "" {
				id = uniqueHeadingID(headingSlug(text), ids)
				n.Attr = append(n.Attr, golanghtml.Attribute{Key: "id", Val: id})
			}
			toc = append(toc, TOCItem{ID: id, Text: text, Level: level})
		case "pre":
			normalizePre(n)
		case "img":
			normalizeImage(n)
		case "a":
			normalizeLink(n)
		case "wbr":
			// These are artifacts of old Word/Live Writer HTML.
		case "span", "div":
			if len(n.Attr) == 0 && strings.TrimSpace(nodeText(n)) == "" {
				return
			}
			n.Attr = cleanStyleAttr(n.Attr)
		default:
			n.Attr = cleanStyleAttr(n.Attr)
		}
	})

	removeArtifacts(body)
	wrapTables(body)

	var buf bytes.Buffer
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		if err := golanghtml.Render(&buf, child); err != nil {
			return source, toc
		}
	}
	return buf.String(), toc
}

func normalizePre(n *golanghtml.Node) {
	lang := ""
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, "brush:") {
			for _, class := range strings.Fields(a.Val) {
				if strings.HasPrefix(class, "brush:") {
					lang = strings.TrimSuffix(strings.TrimPrefix(class, "brush:"), ";")
					break
				}
			}
			// Drop the whole legacy highlighter class attribute.
			continue
		}
		out = append(out, a)
	}
	n.Attr = out

	// Ensure a <code> child exists so the theme can style code blocks.
	var code *golanghtml.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == golanghtml.ElementNode && c.Data == "code" {
			code = c
			break
		}
	}
	if code == nil {
		code = &golanghtml.Node{Type: golanghtml.ElementNode, Data: "code"}
		moveChildren(n, code)
		n.AppendChild(code)
	}
	if lang != "" && !hasClass(code, "language-"+lang) {
		code.Attr = append(code.Attr, golanghtml.Attribute{Key: "class", Val: "language-" + lang})
	}
}

func normalizeImage(n *golanghtml.Node) {
	setAttr(n, "loading", "lazy")
	setAttr(n, "decoding", "async")
	setAttr(n, "referrerpolicy", "no-referrer")
	addClass(n, "content-image")

	if src := attr(n, "src"); strings.HasPrefix(src, "http://") {
		// Prevent mixed-content blocking on HTTPS deployments.
		setAttr(n, "src", "https://"+strings.TrimPrefix(src, "http://"))
	}
}

func normalizeLink(n *golanghtml.Node) {
	href := attr(n, "href")
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		addClass(n, "external-link")
		setAttr(n, "rel", "noopener noreferrer")
	}
}

func cleanStyleAttr(attrs []golanghtml.Attribute) []golanghtml.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key == "style" {
			a.Val = cleanStyle(a.Val)
			if a.Val == "" {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func dropSizeAttrs(attrs []golanghtml.Attribute) []golanghtml.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key == "width" || a.Key == "height" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// cleanStyle keeps emphasis-related rules and drops layout rules that fight
// the responsive theme. Dark mode neutralizes colors separately.
func cleanStyle(style string) string {
	var kept []string
	for _, decl := range strings.Split(style, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		prop := strings.ToLower(strings.TrimSpace(strings.SplitN(decl, ":", 2)[0]))
		switch prop {
		case "color", "font-weight", "font-style", "text-align", "text-decoration":
			kept = append(kept, decl)
		}
	}
	return strings.Join(kept, "; ")
}

// removeArtifacts drops tags left behind by old Windows Live Writer posts.
func removeArtifacts(body *golanghtml.Node) {
	var remove []*golanghtml.Node
	walk(body, func(n *golanghtml.Node) {
		if n.Type != golanghtml.ElementNode {
			return
		}
		if n.Data == "wbr" {
			remove = append(remove, n)
			return
		}
		if n.Data == "span" && len(n.Attr) == 0 && n.FirstChild == nil {
			remove = append(remove, n)
		}
	})
	for _, n := range remove {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
}

func wrapTables(body *golanghtml.Node) {
	var tables []*golanghtml.Node
	walk(body, func(n *golanghtml.Node) {
		if n.Type == golanghtml.ElementNode && n.Data == "table" {
			tables = append(tables, n)
		}
	})
	for _, table := range tables {
		parent := table.Parent
		if parent == nil {
			continue
		}
		wrapper := &golanghtml.Node{
			Type: golanghtml.ElementNode,
			Data: "div",
			Attr: []golanghtml.Attribute{{Key: "class", Val: "table-wrap"}},
		}
		parent.InsertBefore(wrapper, table)
		parent.RemoveChild(table)
		wrapper.AppendChild(table)
	}
}

func findNode(n *golanghtml.Node, tag string) *golanghtml.Node {
	if n.Type == golanghtml.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func walk(n *golanghtml.Node, fn func(*golanghtml.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func nodeText(n *golanghtml.Node) string {
	if n.Type == golanghtml.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

func attr(n *golanghtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *golanghtml.Node, key, value string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, golanghtml.Attribute{Key: key, Val: value})
}

func addClass(n *golanghtml.Node, class string) {
	for i, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		for _, existing := range strings.Fields(a.Val) {
			if existing == class {
				return
			}
		}
		n.Attr[i].Val = strings.TrimSpace(a.Val + " " + class)
		return
	}
	n.Attr = append(n.Attr, golanghtml.Attribute{Key: "class", Val: class})
}

func hasClass(n *golanghtml.Node, class string) bool {
	for _, existing := range strings.Fields(attr(n, "class")) {
		if existing == class {
			return true
		}
	}
	return false
}

func moveChildren(from, to *golanghtml.Node) {
	for c := from.FirstChild; c != nil; {
		next := c.NextSibling
		from.RemoveChild(c)
		to.AppendChild(c)
		c = next
	}
}

func headingSlug(text string) string {
	slug := model.Slugify(text)
	if slug == "" {
		return "section"
	}
	if len([]rune(slug)) > 80 {
		slug = string([]rune(slug)[:80])
	}
	return slug
}

func uniqueHeadingID(base string, seen map[string]int) string {
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return base
	}
	return base + "-" + strconv.Itoa(n+1)
}
