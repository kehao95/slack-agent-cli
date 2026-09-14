package eventstore

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLegacyEventLabelsNeverBecomeUsernames(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	// Reproduce the old event_json shape, including an untrusted display label.
	cursor, err := store.Insert(ctx, Event{
		Kind: "slack.event", Type: "reaction_added",
		User: "@Alice Example", UserID: "U123",
		ItemUser: "@bob", ItemUserID: "U456",
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.Query(ctx, Filter{Limit: 10})
	if err != nil || len(events) != 1 {
		t.Fatalf("query = %v, %v", events, err)
	}
	event := events[0]
	if event.Cursor != cursor || event.User != "U123" || event.ItemUser != "U456" || event.Username != "" || event.ItemUsername != "" {
		t.Fatalf("old label promoted to identity: %+v", event)
	}
}
