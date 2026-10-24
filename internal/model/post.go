package model

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Post types.
const (
	PostTypePost = "post"
	PostTypePage = "page"
)

// Post statuses.
const (
	StatusPublish = "publish"
	StatusDraft   = "draft"
	StatusHidden  = "hidden"
	StatusWaiting = "waiting"
)

// Content formats.
const (
	FormatMarkdown = "markdown"
	FormatHTML     = "html"
)

// Post is a blog entry or an independent page.
type Post struct {
	ID      int64  `json:"id"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Excerpt string `json:"excerpt"`
	Cover   string `json:"cover,omitempty"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	// Format is "markdown" or "html". Imported Typecho posts may be raw HTML.
	Format string `json:"format"`
	// Template is the legacy Typecho page template name, kept for reference.
	Template string `json:"template,omitempty"`
	AuthorID *int64 `json:"author_id,omitempty"`
	// Author is the display name resolved from the linked user, falling back
	// to the site author option.
	Author       string `json:"author"`
	PostPassword string `json:"-"`
	AllowComment bool   `json:"allow_comment"`
	Pinned       bool   `json:"pinned"`
	Views        int64  `json:"views"`
	Likes        int64  `json:"likes"`

	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Categories []*Category `json:"categories,omitempty"`
	Tags       []*Tag      `json:"tags,omitempty"`

	// CategoryIDs and TagIDs are accepted on create/update. A nil slice means
	// "leave relations unchanged" on update; an empty slice clears them.
	CategoryIDs []int64 `json:"category_ids,omitempty"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`

	// Source and SourceID identify imported content (for example Typecho
	// cid values). They are internal and not exposed over JSON.
	Source   string `json:"-"`
	SourceID string `json:"-"`
}

// DisplayTime returns the time a post should be presented with: its
// publication time when set, otherwise its creation time.
func (p *Post) DisplayTime() time.Time {
	if p.PublishedAt != nil {
		return *p.PublishedAt
	}
	return p.CreatedAt
}

// IsLocked reports whether the post requires a password.
func (p *Post) IsLocked() bool {
	return p.PostPassword != ""
}

// IsPublished reports whether the post has publish status.
func (p *Post) IsPublished() bool {
	return p.Status == StatusPublish
}

// PubliclyVisible reports whether an anonymous visitor may read the post.
// Hidden posts are reachable by URL but omitted from listings; scheduled
// posts become visible at their publication time.
func (p *Post) PubliclyVisible(now time.Time) bool {
	switch p.Status {
	case StatusHidden:
		return true
	case StatusPublish:
		return p.PublishedAt == nil || !p.PublishedAt.After(now)
	default:
		return false
	}
}

var (
	mdFenceRe  = regexp.MustCompile("(?s)```.*?```")
	mdImageRe  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLinkRe   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdTagRe    = regexp.MustCompile(`<[^>]+>`)
	mdPrefixRe = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}|>|[-*+]|\d+\.)\s*`)
	mdMarksRe  = regexp.MustCompile("[*_~`]+")
	spaceRe    = regexp.MustCompile(`\s+`)
)

// MakeExcerpt builds a plain-text summary from markdown content.
func MakeExcerpt(content string, limit int) string {
	s := PlainText(content)

	if limit <= 0 {
		limit = 200
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}

	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// PlainText strips common markdown syntax from content.
func PlainText(content string) string {
	s := mdFenceRe.ReplaceAllString(content, " ")
	s = mdImageRe.ReplaceAllString(s, " ")
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = mdTagRe.ReplaceAllString(s, " ")
	s = mdPrefixRe.ReplaceAllString(s, "")
	s = mdMarksRe.ReplaceAllString(s, "")
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}
