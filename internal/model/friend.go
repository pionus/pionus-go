package model

import (
	"strings"
	"time"
	"unicode/utf8"
)

// FriendLink is a managed friend link on the public links page.
type FriendLink struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Avatar      string    `json:"avatar"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	Visible     bool      `json:"visible"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Initial returns the fallback avatar character.
func (f *FriendLink) Initial() string {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(name)
	return strings.ToUpper(string(r))
}
