package model

import (
	"strings"
	"time"
	"unicode"
)

// Category groups posts. Categories may be nested via ParentID.
type Category struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	ParentID    *int64    `json:"parent_id,omitempty"`
	SortOrder   int       `json:"sort_order"`
	PostCount   int       `json:"post_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// Tag labels posts.
type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	PostCount int       `json:"post_count"`
	CreatedAt time.Time `json:"created_at"`
}

// Slugify turns a human-readable name into a URL-safe-ish slug. Unicode
// letters are kept, so Chinese titles remain readable in URLs.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
