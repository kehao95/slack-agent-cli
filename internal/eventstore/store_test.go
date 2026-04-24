package eventstore

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStoreInsertQueryAndLatestCursor(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	first, err := store.Insert(ctx, Event{
		Kind:             "slack.event",
		Type:             "message",
		ChannelID:        "C123",
		ConversationType: "channel",
		UserID:           "U123",
		TS:               "1776957488.000100",
		ThreadTS:         "1776957488.000000",
		Text:             "hello",
		IsThreadReply:    true,
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	second, err := store.Insert(ctx, Event{
		Kind:             "slack.event",
		Type:             "message",
		ChannelID:        "C123",
		ConversationType: "channel",
		UserID:           "U999",
		TS:               "1776957499.000100",
		Text:             "self",
		IsSelf:           true,
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	if second <= first {
		t.Fatalf("expected cursor to increase, got first=%d second=%d", first, second)
	}

	events, err := store.Query(ctx, Filter{
		ChannelID:   "C123",
		ThreadTS:    "1776957488.000000",
		ThreadsOnly: true,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one thread event, got %d", len(events))
	}
	if events[0].Cursor != first || events[0].Text != "hello" {
		t.Fatalf("unexpected event: %+v", events[0])
	}

	events, err = store.Query(ctx, Filter{SinceCursor: first, ExcludeSelf: true, Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected self event to be filtered, got %d", len(events))
	}

	latest, err := store.LatestCursor(ctx)
	if err != nil {
		t.Fatalf("LatestCursor returned error: %v", err)
	}
	if latest != second {
		t.Fatalf("expected latest cursor %d, got %d", second, latest)
	}

	events, err = store.Query(ctx, Filter{Type: "reaction_added", Limit: 10})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no reaction events, got %d", len(events))
	}
}

func TestStorePruneOlderThan(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := store.Insert(ctx, Event{Kind: "slack.event", Type: "message", ReceivedAt: now.Add(-2 * time.Hour)}); err != nil {
		t.Fatalf("Insert old returned error: %v", err)
	}
	if _, err := store.Insert(ctx, Event{Kind: "slack.event", Type: "message", ReceivedAt: now}); err != nil {
		t.Fatalf("Insert new returned error: %v", err)
	}

	deleted, err := store.PruneOlderThan(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("PruneOlderThan returned error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one event pruned, got %d", deleted)
	}
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one retained event, got %d", count)
	}
}

func TestConcurrentOpenAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if _, err := store.Insert(context.Background(), Event{Kind: "slack.event", Type: "message", ChannelID: "C123"}); err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader, err := Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer reader.Close()
			if _, err := reader.Query(context.Background(), Filter{Limit: 1}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent open/read returned error: %v", err)
	}
}

func TestClaimAndAckLifecycle(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	cursor, err := store.Insert(ctx, Event{
		Kind:   "slack.event",
		Type:   "message",
		TS:     "1776998000.000100",
		Text:   "hello from queue",
		UserID: "U123",
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	claimed, ok, err := store.Claim(ctx, Filter{
		Type:  "message",
		Limit: 1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected one claimed event, got none")
	}
	if claimed.Cursor != cursor {
		t.Fatalf("expected cursor %d, got %d", cursor, claimed.Cursor)
	}

	again, ok, err := store.Claim(ctx, Filter{
		Type:  "message",
		Limit: 1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("second Claim returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected no event while lease is valid, got cursor %d", again.Cursor)
	}

	acked, err := store.Ack(ctx, claimed.Cursor)
	if err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
	if !acked {
		t.Fatalf("expected acked=true")
	}

	afterAck, ok, err := store.Claim(ctx, Filter{
		Type:  "message",
		Limit: 1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim after ack returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected no claimable event after ack, got cursor %d", afterAck.Cursor)
	}

	if _, err := store.Ack(ctx, 99999); err == nil {
		t.Fatal("expected not found for invalid cursor")
	}
}

func TestClaimExpiresAndCanReclaim(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	cursor, err := store.Insert(ctx, Event{
		Kind: "slack.event",
		Type: "message",
		TS:   "1776998000.000200",
		Text: "will expire",
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	claimed, ok, err := store.Claim(ctx, Filter{
		Type:  "message",
		Limit: 1,
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}
	if !ok || claimed.Cursor != cursor {
		t.Fatalf("expected claimed cursor %d, got ok=%t cursor=%d", cursor, ok, claimed.Cursor)
	}

	time.Sleep(40 * time.Millisecond)
	reclaimed, ok, err := store.Claim(ctx, Filter{
		Type:  "message",
		Limit: 1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim after expiry returned error: %v", err)
	}
	if !ok || reclaimed.Cursor != cursor {
		t.Fatalf("expected reclaim cursor %d, got ok=%t cursor=%d", cursor, ok, reclaimed.Cursor)
	}
}

func TestClaimMessageKindFilters(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	rootCursor, err := store.Insert(ctx, Event{
		Kind:      "slack.event",
		Type:      "message",
		ChannelID: "C123",
		TS:        "1776999000.000100",
		Text:      "root task",
	})
	if err != nil {
		t.Fatalf("Insert root returned error: %v", err)
	}
	replyCursor, err := store.Insert(ctx, Event{
		Kind:          "slack.event",
		Type:          "message",
		ChannelID:     "C123",
		TS:            "1776999001.000100",
		ThreadTS:      "1776999000.000100",
		Text:          "thread reply",
		IsThreadReply: true,
	})
	if err != nil {
		t.Fatalf("Insert reply returned error: %v", err)
	}

	root, ok, err := store.Claim(ctx, Filter{
		Type:        "message",
		MessageKind: "root",
		ChannelID:   "C123",
		Limit:       1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim root returned error: %v", err)
	}
	if !ok || root.Cursor != rootCursor {
		t.Fatalf("expected root cursor %d, got ok=%t cursor=%d", rootCursor, ok, root.Cursor)
	}

	reply, ok, err := store.Claim(ctx, Filter{
		Type:        "message",
		MessageKind: "reply",
		ChannelID:   "C123",
		Limit:       1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim reply returned error: %v", err)
	}
	if !ok || reply.Cursor != replyCursor {
		t.Fatalf("expected reply cursor %d, got ok=%t cursor=%d", replyCursor, ok, reply.Cursor)
	}
}

func TestClaimMentionsMeFilter(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.Insert(ctx, Event{
		Kind: "slack.event",
		Type: "message",
		Text: "hello <@U999>",
	}); err != nil {
		t.Fatalf("Insert non-mention returned error: %v", err)
	}
	mentionCursor, err := store.Insert(ctx, Event{
		Kind: "slack.event",
		Type: "message",
		Text: "hello <@U123> please check this",
	})
	if err != nil {
		t.Fatalf("Insert mention returned error: %v", err)
	}

	claimed, ok, err := store.Claim(ctx, Filter{
		Type:          "message",
		MentionUserID: "U123",
		Limit:         1,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Claim mention returned error: %v", err)
	}
	if !ok || claimed.Cursor != mentionCursor {
		t.Fatalf("expected mention cursor %d, got ok=%t cursor=%d", mentionCursor, ok, claimed.Cursor)
	}
}
