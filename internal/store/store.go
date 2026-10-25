package store

import (
	"context"
	"errors"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

// Common store errors. Handlers should match them with errors.Is.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// Store is the full persistence interface used by the application.
type Store interface {
	PostStore
	TaxonomyStore
	CommentStore
	UserStore
	MediaStore
	NoteStore
	OptionStore
	Ping(ctx context.Context) error
	Close() error
}

// PostFilter selects and shapes post listings.
type PostFilter struct {
	// Type restricts results to a post type (post/page). Empty means any.
	Type string
	// Statuses restricts results to the given statuses. Empty means any.
	Statuses []string
	// CategorySlug and TagSlug filter by taxonomy slug.
	CategorySlug string
	TagSlug      string
	// Query is a case-insensitive substring match on title and content.
	Query string
	// PublishedBefore hides posts scheduled after the given time.
	PublishedBefore *time.Time
	// PublishedFrom/PublishedTo select a publication-time range.
	PublishedFrom *time.Time
	PublishedTo   *time.Time
	// Page is 1-based. Limit defaults to 20.
	Page  int
	Limit int
	// WithContent selects the full body. Listings usually leave it false.
	WithContent bool
	// WithTaxonomy loads categories and tags for each post.
	WithTaxonomy bool
}

// PostStore manages posts and pages.
type PostStore interface {
	PostReader
	PostWriter
	PostMetrics
	GetPostByID(id int64) (*model.Post, error)
	GetPostBySource(source, sourceID string) (*model.Post, error)
	// UpdatePostContent rewrites the body without touching timestamps or
	// relations. It is used by migration link repair.
	UpdatePostContent(id int64, content, excerpt string) error
}

// TaxonomyStore manages categories, tags and their post relations.
type TaxonomyStore interface {
	TaxonomyReader
	CreateCategory(c *model.Category) error
	UpdateCategory(slug string, c *model.Category) error
	DeleteCategory(slug string) error

	CreateTag(t *model.Tag) error
	UpdateTag(slug string, t *model.Tag) error
	DeleteTag(slug string) error
	// UpsertTag finds a tag by slug or name, creating it when missing. It is
	// used by importers and the editor's free-form tag input.
	UpsertTag(name, slug string) (*model.Tag, error)

	SetPostCategories(postID int64, categoryIDs []int64) error
	SetPostTags(postID int64, tagIDs []int64) error
	PostCategories(postID int64) ([]*model.Category, error)
	PostTags(postID int64) ([]*model.Tag, error)
}

// CommentFilter selects comment listings.
type CommentFilter struct {
	PostID int64
	Status string
	Page   int
	Limit  int
}

// CommentStore manages post comments.
type CommentStore interface {
	CommentReader
	CommentWriter
	GetCommentBySource(source, sourceID string) (*model.Comment, error)
	UpdateCommentStatus(id int64, status string) error
	// UpdateCommentContent rewrites a comment body for migration link repair.
	UpdateCommentContent(id int64, content string) error
	DeleteComment(id int64) error
	CountComments(postID int64, status string) (int, error)
}

// UserStore manages accounts and sessions.
type UserStore interface {
	CreateUser(u *model.User) error
	GetUser(id int64) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	GetUserByEmail(email string) (*model.User, error)
	ListUsers() ([]*model.User, error)
	UpdateUser(u *model.User) error
	DeleteUser(id int64) error
	TouchUserLogin(id int64, at time.Time) error

	CreateSession(s *model.Session) error
	// GetSession returns a non-expired session and its user.
	GetSession(tokenHash string, now time.Time) (*model.Session, *model.User, error)
	DeleteSession(tokenHash string) error
	DeleteUserSessions(userID int64) error
	DeleteExpiredSessions(now time.Time) error
}

// NoteStore manages short posts shown in the 微语 timeline.
type NoteStore interface {
	NoteReader
	GetNoteBySource(source, sourceID string) (*model.Note, error)
	CreateNote(n *model.Note) error
	UpdateNote(id int64, content, status string) error
	DeleteNote(id int64) error
	CountNotes(status string) (int, error)
}

// MediaStore manages the uploaded-file library.
type MediaStore interface {
	CreateMedia(m *model.Media) error
	GetMedia(id int64) (*model.Media, error)
	ListMedia(page, limit int) ([]*model.Media, int, error)
	DeleteMedia(id int64) error
}

// OptionStore stores site settings as key/value pairs.
type OptionStore interface {
	OptionReader
	SetOption(name, value string) error
	AllOptions() (map[string]string, error)
}
