package channels

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/slack"
)

type resolverMockClient struct {
	responses [][]slackapi.Channel
	cursors   []string
	index     int
	error     error
}

func (m *resolverMockClient) ListConversationsHistory(ctx context.Context, params slack.HistoryParams) (*slackapi.GetConversationHistoryResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *resolverMockClient) ListThreadReplies(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, bool, string, error) {
	return nil, false, "", errors.New("not implemented")
}

func (m *resolverMockClient) ListChannels(ctx context.Context, params slack.ListChannelsParams) ([]slackapi.Channel, string, error) {
	if m.error != nil {
		return nil, "", m.error
	}
	if m.index >= len(m.responses) {
		return nil, "", nil
	}
	resp := m.responses[m.index]
	cursor := ""
	if m.index < len(m.cursors) {
		cursor = m.cursors[m.index]
	}
	m.index++
	return resp, cursor, nil
}

func TestResolverResolveID_DirectID(t *testing.T) {
	// Direct channel IDs should work without cache
	resolver := NewResolver(nil)
	id, err := resolver.ResolveID(context.Background(), "C123ABC")
	if err != nil {
		t.Fatalf("ResolveID returned error: %v", err)
	}
	if id != "C123ABC" {
		t.Fatalf("expected C123ABC, got %s", id)
	}
}

func TestResolverResolveID_NoCacheFallback(t *testing.T) {
	// Without cache, resolver falls back to direct API fetch
	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "general", Conversation: slackapi.Conversation{ID: "C1"}}}},
		},
	}
	resolver := NewResolver(client)
	id, err := resolver.ResolveID(context.Background(), "#general")
	if err != nil {
		t.Fatalf("ResolveID returned error: %v", err)
	}
	if id != "C1" {
		t.Fatalf("expected C1, got %s", id)
	}
}

func TestResolverResolveIDError(t *testing.T) {
	client := &resolverMockClient{error: errors.New("boom")}
	resolver := NewResolver(client)
	_, err := resolver.ResolveID(context.Background(), "#missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolverCacheHit(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate the cache (simulating "cache populate" command)
	channels := []slackapi.Channel{
		{GroupConversation: slackapi.GroupConversation{Name: "cached", Conversation: slackapi.Conversation{ID: "C2"}}},
	}
	if err := store.Save(cache.CacheKeyChannels, channels); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &resolverMockClient{} // No responses needed - should hit cache
	resolver := NewCachedResolver(client, store)

	id, err := resolver.ResolveID(context.Background(), "#cached")
	if err != nil {
		t.Fatalf("ResolveID returned error: %v", err)
	}
	if id != "C2" {
		t.Fatalf("expected C2, got %s", id)
	}
	if client.index != 0 {
		t.Fatalf("expected no API calls (cache hit), got %d", client.index)
	}
}

func TestResolverCacheIncomplete(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate with partial cache (incomplete)
	channels := []slackapi.Channel{
		{GroupConversation: slackapi.GroupConversation{Name: "known", Conversation: slackapi.Conversation{ID: "C1"}}},
	}
	if err := store.SavePartial(cache.CacheKeyChannels, channels, "next_cursor", false, 1); err != nil {
		t.Fatalf("failed to save partial cache: %v", err)
	}

	client := &resolverMockClient{}
	resolver := NewCachedResolver(client, store)

	// Known channel should resolve from partial cache
	id, err := resolver.ResolveID(context.Background(), "#known")
	if err != nil {
		t.Fatalf("ResolveID #known returned error: %v", err)
	}
	if id != "C1" {
		t.Fatalf("expected C1, got %s", id)
	}

	// Unknown channel should return ErrCacheIncomplete
	_, err = resolver.ResolveID(context.Background(), "#unknown")
	if err == nil {
		t.Fatal("expected error for unknown channel with incomplete cache")
	}
	var incompleteErr ErrCacheIncomplete
	if !errors.As(err, &incompleteErr) {
		t.Fatalf("expected ErrCacheIncomplete, got %T: %v", err, err)
	}
	if incompleteErr.CachedCount != 1 {
		t.Errorf("expected CachedCount=1, got %d", incompleteErr.CachedCount)
	}
}

func TestResolverRefreshCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate cache
	channels := []slackapi.Channel{
		{GroupConversation: slackapi.GroupConversation{Name: "old", Conversation: slackapi.Conversation{ID: "C3"}}},
	}
	if err := store.Save(cache.CacheKeyChannels, channels); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &resolverMockClient{}
	resolver := NewCachedResolver(client, store)

	// Verify cache works
	_, err := resolver.ResolveID(context.Background(), "#old")
	if err != nil {
		t.Fatalf("ResolveID #old returned error: %v", err)
	}

	// RefreshCache clears the cache
	if err := resolver.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache returned error: %v", err)
	}

	// After refresh, cache is empty - should return ErrCacheIncomplete
	_, err = resolver.ResolveID(context.Background(), "#old")
	if err == nil {
		t.Fatal("expected error after cache refresh")
	}
	var incompleteErr ErrCacheIncomplete
	if !errors.As(err, &incompleteErr) {
		t.Fatalf("expected ErrCacheIncomplete after refresh, got %T: %v", err, err)
	}
}
