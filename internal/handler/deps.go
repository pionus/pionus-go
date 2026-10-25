package handler

import (
	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/store"
)

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
	// Events receives domain events after successful writes. It defaults to a
	// no-op emitter so tests and embedded uses can omit plugins.
	Events events.Emitter
}

// NewDeps wires a full store into the public handler dependencies. The
// optional emitter is normally the plugin manager.
func NewDeps(db store.Store, emitters ...events.Emitter) Deps {
	var emitter events.Emitter = events.Nop{}
	if len(emitters) > 0 && emitters[0] != nil {
		emitter = emitters[0]
	}
	return Deps{
		Posts:         db,
		PostWrites:    db,
		Views:         db,
		Taxonomy:      db,
		Comments:      db,
		CommentWrites: db,
		Notes:         db,
		Options:       db,
		Events:        emitter,
	}
}

// Emit publishes an event with the request context. It is nil-safe for Deps
// constructed directly in tests.
func (d Deps) Emit(ctx arry.Context, event events.Event) {
	if d.Events == nil {
		return
	}
	d.Events.Emit(ctx.Request().Context(), event)
}
