// Package events defines the application event envelope and the interfaces
// used by public/admin handlers to publish events to extensions.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

// Event schema version. Bump it when the envelope changes incompatibly.
const Version = 1

// Event types emitted by the core application.
const (
	PostCreated          Type = "post.created"
	PostUpdated          Type = "post.updated"
	PostDeleted          Type = "post.deleted"
	CommentCreated       Type = "comment.created"
	CommentStatusChanged Type = "comment.status_changed"
	CommentDeleted       Type = "comment.deleted"
	NoteCreated          Type = "note.created"
	NoteUpdated          Type = "note.updated"
	NoteDeleted          Type = "note.deleted"
)

// Event sources identify which part of the application produced an event.
const (
	SourceWeb    = "web"
	SourceAPI    = "api"
	SourceAdmin  = "admin"
	SourceSystem = "system"
)

// Type identifies an event.
type Type string

// Event is the envelope delivered to extensions.
type Event struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	Source     string    `json:"source"`
	Data       any       `json:"data"`
}

// PostData is the payload for post.* events.
type PostData struct {
	Post *model.Post `json:"post"`
}

// CommentData is the payload for comment.created.
type CommentData struct {
	Comment *model.Comment `json:"comment"`
}

// CommentStatusData is the payload for comment.status_changed.
type CommentStatusData struct {
	Comment *model.Comment `json:"comment"`
	Status  string         `json:"status"`
}

// NoteData is the payload for note.created and note.updated.
type NoteData struct {
	Note *model.Note `json:"note"`
}

// DeletedData is the payload for events whose entity no longer exists.
type DeletedData struct {
	ID   int64  `json:"id,omitempty"`
	Slug string `json:"slug,omitempty"`
}

// Emitter publishes application events. Implementations must not block the
// caller for longer than their documented budget; the plugin manager wraps
// each handler with a timeout.
type Emitter interface {
	Emit(ctx context.Context, event Event)
}

// Handler consumes application events. Handlers are called sequentially in
// plugin registration order and must be safe for concurrent event delivery.
type Handler interface {
	HandleEvent(ctx context.Context, event Event) error
}

// Nop is an Emitter that discards all events. It is the default in tests and
// in configurations without plugins.
type Nop struct{}

// Emit implements Emitter.
func (Nop) Emit(context.Context, Event) {}

// New builds an event with a random ID, current time and the current schema
// version.
func New(eventType Type, source string, data any) Event {
	return Event{
		ID:         newID(),
		Type:       eventType,
		Version:    Version,
		OccurredAt: time.Now().UTC(),
		Source:     source,
		Data:       data,
	}
}

func newID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	// crypto/rand failing is exceptional; keep the event usable.
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
