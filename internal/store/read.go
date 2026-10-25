package store

import "github.com/pionus/pionus-go/internal/model"

// PostReader is the post access needed by public pages and API reads.
type PostReader interface {
	GetPost(slug string) (*model.Post, error)
	ListPosts(f PostFilter) ([]*model.Post, int, error)
	AdjacentPosts(slug string) (prev, next *model.Post, err error)
}

// PostWriter covers post mutations exposed by the JSON API.
type PostWriter interface {
	CreatePost(p *model.Post) error
	UpdatePost(slug string, p *model.Post) error
	DeletePost(slug string) error
}

// PostMetrics records public post views.
type PostMetrics interface {
	IncrementPostViews(slug string) error
}

// TaxonomyReader is the taxonomy access needed by public handlers.
type TaxonomyReader interface {
	ListCategories() ([]*model.Category, error)
	GetCategory(slug string) (*model.Category, error)
	ListTags() ([]*model.Tag, error)
	GetTag(slug string) (*model.Tag, error)
}

// CommentReader is the comment access needed by public handlers.
type CommentReader interface {
	GetComment(id int64) (*model.Comment, error)
	ListComments(f CommentFilter) ([]*model.Comment, int, error)
}

// CommentWriter creates comments from the public form.
type CommentWriter interface {
	CreateComment(c *model.Comment) error
}

// NoteReader is the note access needed by the public timeline.
type NoteReader interface {
	ListNotes(f NoteFilter) ([]*model.Note, int, error)
	GetNote(id int64) (*model.Note, error)
}

// OptionReader is the settings access needed by public handlers.
type OptionReader interface {
	GetOption(name string) (string, error)
}

// FriendLinkReader lists managed friend links.
type FriendLinkReader interface {
	ListFriendLinks(includeHidden bool) ([]*model.FriendLink, error)
}
