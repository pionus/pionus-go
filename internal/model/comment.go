package model

import "time"

// Comment statuses.
const (
	CommentPending  = "pending"
	CommentApproved = "approved"
	CommentSpam     = "spam"
)

// Comment is a reader comment on a post.
type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	ParentID  *int64    `json:"parent_id,omitempty"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	IP        string    `json:"-"`
	UserAgent string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`

	// PostTitle and PostSlug are filled by admin listing queries.
	PostTitle string `json:"post_title,omitempty"`
	PostSlug  string `json:"post_slug,omitempty"`

	// PostLocked is set by listing queries when the parent post requires a
	// password.
	PostLocked bool `json:"-"`

	// Source and SourceID identify imported comments.
	Source   string `json:"-"`
	SourceID string `json:"-"`
}
