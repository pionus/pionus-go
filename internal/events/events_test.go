package events

import (
	"context"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	event := New(PostCreated, SourceAPI, PostData{})
	if event.ID == "" {
		t.Fatal("event id is empty")
	}
	if event.Type != PostCreated || event.Source != SourceAPI {
		t.Fatalf("event = %+v", event)
	}
	if event.Version != Version {
		t.Fatalf("version = %d, want %d", event.Version, Version)
	}
	if event.OccurredAt.IsZero() || time.Since(event.OccurredAt) > time.Minute {
		t.Fatalf("occurred_at = %v", event.OccurredAt)
	}
}

func TestNopEmitter(t *testing.T) {
	var emitter Emitter = Nop{}
	emitter.Emit(context.Background(), New(NoteCreated, SourceAdmin, NoteData{}))
}
