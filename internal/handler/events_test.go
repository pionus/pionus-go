package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/pionus/arry"
	"github.com/pionus/arry/middlewares"
	"github.com/pionus/pionus-go/internal/events"
	"github.com/pionus/pionus-go/internal/store"
)

type captureEmitter struct {
	events []events.Event
}

func (c *captureEmitter) Emit(_ context.Context, event events.Event) {
	c.events = append(c.events, event)
}

func newEventHandler(t *testing.T, emitter events.Emitter) (*arry.Arry, store.Store) {
	t.Helper()

	db, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	app := arry.New()
	app.Use(middlewares.Auth("secret", http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete))
	deps := NewDeps(db, emitter)
	app.Router().Post("/api/posts", CreatePost(deps))
	app.Router().Put("/api/posts/:slug", UpdatePost(deps))
	app.Router().Delete("/api/posts/:slug", DeletePost(deps))
	return app, db
}

func TestAPIEmitsPostEvents(t *testing.T) {
	emitter := &captureEmitter{}
	app, _ := newEventHandler(t, emitter)

	rec := doRequest(t, app, http.MethodPost, "/api/posts", "secret",
		`{"title":"Event Post","content":"body","slug":"event-post"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(emitter.events) != 1 {
		t.Fatalf("events = %+v", emitter.events)
	}
	created := emitter.events[0]
	if created.Type != events.PostCreated || created.Source != events.SourceAPI {
		t.Fatalf("event = %+v", created)
	}
	if data, ok := created.Data.(events.PostData); !ok || data.Post == nil || data.Post.Slug != "event-post" {
		t.Fatalf("data = %#v", created.Data)
	}

	rec = doRequest(t, app, http.MethodPut, "/api/posts/event-post", "secret",
		`{"title":"Renamed","content":"body"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(emitter.events) != 2 || emitter.events[1].Type != events.PostUpdated {
		t.Fatalf("events = %+v", emitter.events)
	}

	rec = doRequest(t, app, http.MethodDelete, "/api/posts/event-post", "secret", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(emitter.events) != 3 || emitter.events[2].Type != events.PostDeleted {
		t.Fatalf("events = %+v", emitter.events)
	}
	if data, ok := emitter.events[2].Data.(events.PostData); !ok || data.Post == nil || data.Post.Slug != "event-post" {
		t.Fatalf("delete data = %#v", emitter.events[2].Data)
	}
}
