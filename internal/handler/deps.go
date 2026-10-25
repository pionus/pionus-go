package handler

import "github.com/pionus/pionus-go/internal/store"

// Deps is the narrow persistence surface used by public pages, feeds and the
// JSON API. Each handler reads only the fields it needs; admin code keeps the
// full store because it exposes every management operation.
type Deps struct {
	Posts         store.PostReader
	PostWrites    store.PostWriter
	Views         store.PostMetrics
	Taxonomy      store.TaxonomyReader
	Comments      store.CommentReader
	CommentWrites store.CommentWriter
	Notes         store.NoteReader
	Options       store.OptionReader
}

// NewDeps wires a full store into the public handler dependencies.
func NewDeps(db store.Store) Deps {
	return Deps{
		Posts:         db,
		PostWrites:    db,
		Views:         db,
		Taxonomy:      db,
		Comments:      db,
		CommentWrites: db,
		Notes:         db,
		Options:       db,
	}
}
