// Package content holds domain rules shared by the JSON API and the admin UI.
package content

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

var slugRe = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

// ErrTitleRequired is returned when a post has no title.
var ErrTitleRequired = errors.New("title is required")

// PostInput is the editable representation of a post. Pointer fields let an
// update distinguish "not provided" from a zero value.
type PostInput struct {
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	Excerpt      string     `json:"excerpt"`
	Cover        string     `json:"cover"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	AuthorID     *int64     `json:"author_id"`
	Password     string     `json:"password"`
	AllowComment *bool      `json:"allow_comment"`
	Pinned       *bool      `json:"pinned"`
	PublishedAt  *time.Time `json:"published_at"`
	CreatedAt    *time.Time `json:"created_at"`
	CategoryIDs  []int64    `json:"category_ids"`
	TagIDs       []int64    `json:"tag_ids"`
}

// Build validates the input and produces a model. When existing is nil the
// post is being created; otherwise unspecified fields keep their values.
func (in *PostInput) Build(existing *model.Post) (*model.Post, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	if existing == nil {
		slug := strings.TrimSpace(in.Slug)
		if slug == "" {
			slug = model.Slugify(title)
		}
		if slug == "" || !slugRe.MatchString(slug) {
			return nil, fmt.Errorf("slug must contain only letters, numbers, hyphens, and underscores")
		}
		if strings.TrimSpace(in.Content) == "" {
			return nil, fmt.Errorf("content is required")
		}
		in.Slug = slug
	}

	p := &model.Post{
		Title:        title,
		Content:      in.Content,
		Excerpt:      strings.TrimSpace(in.Excerpt),
		Cover:        in.Cover,
		Type:         in.Type,
		Status:       in.Status,
		AuthorID:     in.AuthorID,
		PostPassword: in.Password,
		CategoryIDs:  in.CategoryIDs,
		TagIDs:       in.TagIDs,
	}

	if existing != nil {
		p.ID = existing.ID
		p.Slug = existing.Slug
		p.CreatedAt = existing.CreatedAt
		p.Views = existing.Views
		p.AuthorID = existing.AuthorID
		p.PublishedAt = existing.PublishedAt
		p.AllowComment = existing.AllowComment
		p.Pinned = existing.Pinned
		if p.Type == "" {
			p.Type = existing.Type
		}
		if p.Status == "" {
			p.Status = existing.Status
		}
		if in.AuthorID != nil {
			p.AuthorID = in.AuthorID
		}
	} else {
		p.Slug = in.Slug
		if p.Status == "" {
			p.Status = model.StatusPublish
		}
		p.AllowComment = true
	}

	if p.Type == "" {
		p.Type = model.PostTypePost
	}
	if p.Status == "" {
		p.Status = model.StatusDraft
	}
	if !ValidType(p.Type) {
		return nil, fmt.Errorf("invalid post type")
	}
	if !ValidStatus(p.Status) {
		return nil, fmt.Errorf("invalid post status")
	}

	if in.AllowComment != nil {
		p.AllowComment = *in.AllowComment
	}
	if in.Pinned != nil {
		p.Pinned = *in.Pinned
	}

	if in.PublishedAt != nil {
		p.PublishedAt = in.PublishedAt
	}
	if existing == nil && in.CreatedAt != nil {
		p.CreatedAt = *in.CreatedAt
	}

	if p.Excerpt == "" {
		p.Excerpt = model.MakeExcerpt(p.Content, 200)
	}

	return p, nil
}

// ValidType reports whether t is a supported post type.
func ValidType(t string) bool {
	return t == model.PostTypePost || t == model.PostTypePage
}

// ValidStatus reports whether s is a supported post status.
func ValidStatus(s string) bool {
	switch s {
	case model.StatusPublish, model.StatusDraft, model.StatusHidden, model.StatusWaiting:
		return true
	}
	return false
}

// ValidSlug reports whether slug is safe to use in a URL path.
func ValidSlug(slug string) bool {
	return slug != "" && slugRe.MatchString(slug)
}
