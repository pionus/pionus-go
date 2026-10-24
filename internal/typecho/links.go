package typecho

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// LinkFixer rewrites old Typecho links in imported content to the new routes.
type LinkFixer struct {
	siteURL   string
	host      string
	postSlugs map[int64]string
	pageSlugs map[string]string
}

// NewLinkFixer creates a fixer for the given old site URL.
func NewLinkFixer(siteURL string) *LinkFixer {
	f := &LinkFixer{
		siteURL:   strings.TrimRight(siteURL, "/"),
		postSlugs: make(map[int64]string),
		pageSlugs: make(map[string]string),
	}
	if u, err := url.Parse(f.siteURL); err == nil {
		f.host = u.Host
	}
	return f
}

// SetPostSlug maps a Typecho cid to the new post slug.
func (f *LinkFixer) SetPostSlug(cid int64, slug string) {
	f.postSlugs[cid] = slug
}

// SetPageSlug maps a Typecho page slug to the new slug.
func (f *LinkFixer) SetPageSlug(oldSlug, newSlug string) {
	if oldSlug != "" {
		f.pageSlugs[oldSlug] = newSlug
	}
}

// boundary avoids matching paths that belong to an external host, e.g.
// http://other.example/archives/42.html.
const boundary = `(^|[\s"'(=])`

var (
	oldPostIndexRe = regexp.MustCompile(boundary + `/index\.php/archives/(\d+)/?`)
	oldPostRe      = regexp.MustCompile(boundary + `/archives/(\d+)/?`)
	oldPageIndexRe = regexp.MustCompile(boundary + `/index\.php/([0-9A-Za-z_-]+)\.html`)
	oldPageRe      = regexp.MustCompile(boundary + `/([0-9A-Za-z_-]+)\.html`)
	oldCatPageRe   = regexp.MustCompile(boundary + `/index\.php/category/([^/\s"'<>]+)/(\d+)/?`)
	oldTagPageRe   = regexp.MustCompile(boundary + `/index\.php/tag/([^/\s"'<>]+)/(\d+)/?`)
	oldCatRe       = regexp.MustCompile(boundary + `/index\.php/category/([^/\s"'<>]+)/?`)
	oldTagRe       = regexp.MustCompile(boundary + `/index\.php/tag/([^/\s"'<>]+)/?`)
	oldSearchRe    = regexp.MustCompile(boundary + `/index\.php/search/([^/\s"'<>]+)/?`)
	oldFeedRe      = regexp.MustCompile(boundary + `/index\.php/feed/?`)
)

// Rewrite converts old Typecho addresses to new routes while leaving external
// links alone. It is safe to run repeatedly.
func (f *LinkFixer) Rewrite(text string) string {
	if text == "" {
		return text
	}

	// Make links to the old host relative first. Keep external hosts intact.
	if f.host != "" {
		for _, prefix := range []string{
			"https://" + f.host + "/",
			"http://" + f.host + "/",
			"//" + f.host + "/",
		} {
			text = strings.ReplaceAll(text, prefix, "/")
		}
	}

	text = strings.ReplaceAll(text, "/usr/uploads/", "/uploads/")

	text = replaceLink(oldPostIndexRe, text, func(m []string) (string, bool) {
		cid, _ := strconv.ParseInt(m[2], 10, 64)
		slug, ok := f.postSlugs[cid]
		return m[1] + "/article/" + slug, ok
	})
	text = replaceLink(oldPostRe, text, func(m []string) (string, bool) {
		cid, _ := strconv.ParseInt(m[2], 10, 64)
		slug, ok := f.postSlugs[cid]
		return m[1] + "/article/" + slug, ok
	})
	text = replaceLink(oldPageIndexRe, text, func(m []string) (string, bool) {
		slug, ok := f.pageSlugs[m[2]]
		return m[1] + "/article/" + slug, ok
	})
	text = replaceLink(oldPageRe, text, func(m []string) (string, bool) {
		slug, ok := f.pageSlugs[m[2]]
		return m[1] + "/article/" + slug, ok
	})

	text = replaceLink(oldCatPageRe, text, func(m []string) (string, bool) {
		return m[1] + "/category/" + m[2] + "?page=" + m[3], true
	})
	text = replaceLink(oldTagPageRe, text, func(m []string) (string, bool) {
		return m[1] + "/tag/" + m[2] + "?page=" + m[3], true
	})
	text = replaceLink(oldCatRe, text, func(m []string) (string, bool) {
		return m[1] + "/category/" + m[2], true
	})
	text = replaceLink(oldTagRe, text, func(m []string) (string, bool) {
		return m[1] + "/tag/" + m[2], true
	})
	text = replaceLink(oldSearchRe, text, func(m []string) (string, bool) {
		return m[1] + "/search?q=" + m[2], true
	})
	text = replaceLink(oldFeedRe, text, func(m []string) (string, bool) {
		return m[1] + "/feed", true
	})

	// Bare old entry points in href attributes.
	for _, pair := range [][2]string{
		{`href="/index.php"`, `href="/"`},
		{`href="/index.php/"`, `href="/"`},
		{"href='/index.php'", "href='/'"},
		{"href='/index.php/'", "href='/'"},
	} {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}

	return text
}

func replaceLink(re *regexp.Regexp, text string, build func([]string) (string, bool)) string {
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		replacement, ok := build(parts)
		if !ok {
			return match
		}
		return replacement
	})
}

// FindOldLinks reports old Typecho URL fragments that remain in text.
func (f *LinkFixer) FindOldLinks(text string) []string {
	if text == "" {
		return nil
	}

	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	if f.host != "" {
		for _, prefix := range []string{"//" + f.host + "/", "http://" + f.host + "/", "https://" + f.host + "/"} {
			if strings.Contains(text, prefix) {
				add(prefix)
			}
		}
	}

	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[\s"'(=])(/index\.php[^\s"')]*)`),
		regexp.MustCompile(`(?:^|[\s"'(=])(/usr/uploads/[^\s"')]*)`),
		regexp.MustCompile(`(?:^|[\s"'(=])(/archives/\d+)`),
	} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			add(m[1])
		}
	}
	for oldSlug := range f.pageSlugs {
		if strings.Contains(text, "/"+oldSlug+".html") {
			add("/" + oldSlug + ".html")
		}
	}
	return out
}
