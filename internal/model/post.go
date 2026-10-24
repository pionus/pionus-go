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

// Post is a blog entry or an independent page.
type Post struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Content  string `json:"content,omitempty"`
	Excerpt  string `json:"excerpt"`
	Cover    string `json:"cover,omitempty"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	AuthorID *int64 `json:"author_id,omitempty"`
	// Author is the display name resolved from the linked user, falling back
	// to the site author option.
	Author       string `json:"author"`
	PostPassword string `json:"-"`
	AllowComment bool   `json:"allow_comment"`
	Pinned       bool   `json:"pinned"`
	Views        int64  `json:"views"`

	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Categories []*Category `json:"categories,omitempty"`
	Tags       []*Tag      `json:"tags,omitempty"`

	// CategoryIDs and TagIDs are accepted on create/update. A nil slice means
	// "leave relations unchanged" on update; an empty slice clears them.
	CategoryIDs []int64 `json:"category_ids,omitempty"`
	TagIDs      []int64 `json:"tag_ids,omitempty"`
}

// DisplayTime returns the time a post should be presented with: its
// publication time when set, otherwise its creation time.
func (p *Post) DisplayTime() time.Time {
	if p.PublishedAt != nil {
		return *p.PublishedAt
	}
	return p.CreatedAt
}

// IsPublished reports whether the post is publicly visible.
func (p *Post) IsPublished() bool {
	return p.Status == StatusPublish
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
