package content

import (
	"bytes"
	stdhtml "html"
	"html/template"
	"strconv"
	"strings"
	"unicode"

	"github.com/pionus/pionus-go/internal/model"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
	// InlineMath reports whether the post contains rendered $...$ math.
	InlineMath bool
}

var postMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// mathMarkdown protects inline $...$ spans from Markdown emphasis parsing.
var mathMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithInlineParsers(util.Prioritized(&inlineMathParser{}, 450)),
	),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// inlineMathParser turns $...$ into the explicit \(...\) delimiter. Keeping
// the conversion inside the Markdown parser prevents underscores and stars in
// formulas from being interpreted as emphasis.
type inlineMathParser struct{}

func (p *inlineMathParser) Trigger() []byte { return []byte{'$'} }

func (p *inlineMathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 2 || line[0] != '$' || line[1] == '$' {
		return nil
	}
	if block.PrecendingCharacter() == '$' || block.PrecendingCharacter() == '\\' {
		return nil
	}

	for i := 1; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] != '$' {
			continue
		}
		if i+1 < len(line) && line[i+1] == '$' {
			i++
			continue
		}
		inner := string(line[1:i])
		if !looksLikeInlineMath(inner) {
			return nil
		}
		block.Advance(i + 1)
		value := make([]byte, 0, len(inner)+4)
		value = append(value, '\\', '(')
		value = append(value, stdhtml.EscapeString(inner)...)
		value = append(value, '\\', ')')
		node := ast.NewString(value)
		node.SetRaw(true)
		return node
	}
	return nil
}

// RenderPost converts a post body to display HTML, normalizes legacy markup,
// assigns heading IDs and extracts a table of contents.
func RenderPost(p *model.Post) RenderedPost {
	var raw string
	if p.Format == model.FormatHTML {
		raw = p.Content
	} else {
		var buf bytes.Buffer
		engine := postMarkdown
		if p.Math {
			engine = mathMarkdown
		}
		if err := engine.Convert([]byte(p.Content), &buf); err != nil {
			raw = p.Content
		} else {
			raw = buf.String()
		}
	}

	normalized, toc := normalizeContent(raw, p.Math)
	return RenderedPost{
		HTML:       template.HTML(normalized),
		TOC:        toc,
		HasMath:    hasMath(normalized),
		HasCode:    strings.Contains(normalized, "<pre"),
		InlineMath: p.Math && strings.Contains(normalized, `\(`),
	}
}

func hasMath(rendered string) bool {
	return strings.Contains(rendered, "$$") ||
		strings.Contains(rendered, `\(`) ||
		strings.Contains(rendered, `\[`)
}

// looksLikeInlineMath rejects currency and prose while accepting common LaTeX
// constructs. Structural characters (backslash, ^, _, =, braces) and letters
// are enough; bare numbers are treated as money.
func looksLikeInlineMath(inner string) bool {
	if inner == "" || strings.TrimSpace(inner) != inner ||
		strings.ContainsAny(inner, "$\r\n") {
		return false
	}
	if strings.ContainsAny(inner, `\^_={}[]()`) {
		return true
	}
	hasLetter := false
	for _, r := range inner {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r), r == '_', r == '.', r == ',':
		case strings.ContainsRune("+-*/<>", r):
		default:
			return false
		}
	}
	return hasLetter
}

// rewriteInlineMath converts $...$ in HTML text nodes, skipping code blocks.
func rewriteInlineMath(body *golanghtml.Node) {
	walk(body, func(n *golanghtml.Node) {
		if n.Type != golanghtml.TextNode || hasAncestor(n, "pre", "code", "script", "style", "textarea") {
			return
		}
		n.Data = convertInlineMath(n.Data)
	})
}

func hasAncestor(n *golanghtml.Node, tags ...string) bool {
	for parent := n.Parent; parent != nil; parent = parent.Parent {
		for _, tag := range tags {
			if parent.Data == tag {
				return true
			}
		}
	}
	return false
}

func convertInlineMath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			b.WriteString("$$")
			i += 2
			continue
		}
		j := i + 1
		for j < len(s) {
			if s[j] == '\\' {
				j += 2
				continue
			}
			if s[j] == '$' {
				if j+1 < len(s) && s[j+1] == '$' {
					j += 2
					continue
				}
				break
			}
			j++
		}
		if j < len(s) && looksLikeInlineMath(s[i+1:j]) {
			b.WriteString(`\(`)
			b.WriteString(s[i+1 : j])
			b.WriteString(`\)`)
			i = j + 1
			continue
		}
		b.WriteByte('$')
		i++
	}
	return b.String()
}

// normalizeContent cleans legacy Typecho HTML and builds a TOC.
func normalizeContent(source string, inlineMath bool) (string, []TOCItem) {
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
	if inlineMath {
		rewriteInlineMath(body)
	}

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
