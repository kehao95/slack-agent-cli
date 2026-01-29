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
	// Direct channel IDs should work without cache or client
	resolver := NewResolver(nil)
	id, err := resolver.ResolveID(context.Background(), "C123ABC")
	if err != nil {
		t.Fatalf("ResolveID returned error: %v", err)
	}
	if id != "C123ABC" {
		t.Fatalf("expected C123ABC, got %s", id)
	}
}

func TestResolverResolveID_NoCacheFetchesOnDemand(t *testing.T) {
	// Without cache, resolver fetches from API on demand
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
	if client.index != 1 {
		t.Fatalf("expected 1 API call, got %d", client.index)
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

func TestResolverCacheMiss_FetchesMore(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate with partial cache (incomplete)
	channels := []slackapi.Channel{
		{GroupConversation: slackapi.GroupConversation{Name: "known", Conversation: slackapi.Conversation{ID: "C1"}}},
	}
	if err := store.SavePartial(cache.CacheKeyChannels, channels, "next_cursor", false, 1); err != nil {
		t.Fatalf("failed to save partial cache: %v", err)
	}

	// Mock client will return the unknown channel on the next page
	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "unknown", Conversation: slackapi.Conversation{ID: "C2"}}}},
		},
		cursors: []string{""}, // No more pages after this
	}
	resolver := NewCachedResolver(client, store)

	// Known channel should resolve from partial cache (no API call)
	id, err := resolver.ResolveID(context.Background(), "#known")
	if err != nil {
		t.Fatalf("ResolveID #known returned error: %v", err)
	}
	if id != "C1" {
		t.Fatalf("expected C1, got %s", id)
	}
	if client.index != 0 {
		t.Fatalf("expected 0 API calls for cached channel, got %d", client.index)
	}

	// Unknown channel should trigger fetch and find it
	id, err = resolver.ResolveID(context.Background(), "#unknown")
	if err != nil {
		t.Fatalf("ResolveID #unknown returned error: %v", err)
	}
	if id != "C2" {
		t.Fatalf("expected C2, got %s", id)
	}
	if client.index != 1 {
		t.Fatalf("expected 1 API call to fetch more, got %d", client.index)
	}

	// Cache should now have both channels
	var cached []slackapi.Channel
	found, _ := store.Load(cache.CacheKeyChannels, &cached)
	if !found {
		t.Fatal("expected complete cache after fetching all pages")
	}
	if len(cached) != 2 {
		t.Fatalf("expected 2 cached channels, got %d", len(cached))
	}
}

func TestResolverCacheMiss_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Empty partial cache with cursor
	if err := store.SavePartial(cache.CacheKeyChannels, []slackapi.Channel{}, "cursor1", false, 0); err != nil {
		t.Fatalf("failed to save partial cache: %v", err)
	}

	// Mock returns pages but never has the channel we want
	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "other1", Conversation: slackapi.Conversation{ID: "C1"}}}},
			{{GroupConversation: slackapi.GroupConversation{Name: "other2", Conversation: slackapi.Conversation{ID: "C2"}}}},
		},
		cursors: []string{"cursor2", ""}, // Last page
	}
	resolver := NewCachedResolver(client, store)

	// Should fetch all pages and then return not found
	_, err := resolver.ResolveID(context.Background(), "#nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent channel")
	}
	if !errors.Is(err, nil) && err.Error() != "channel #nonexistent not found" {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.index != 2 {
		t.Fatalf("expected 2 API calls to exhaust pages, got %d", client.index)
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

	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "new", Conversation: slackapi.Conversation{ID: "C4"}}}},
		},
	}
	resolver := NewCachedResolver(client, store)

	// Verify cache works
	id, err := resolver.ResolveID(context.Background(), "#old")
	if err != nil {
		t.Fatalf("ResolveID #old returned error: %v", err)
	}
	if id != "C3" {
		t.Fatalf("expected C3, got %s", id)
	}

	// RefreshCache clears the cache
	if err := resolver.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache returned error: %v", err)
	}

	// After refresh, cache is empty - should fetch from API
	_, err = resolver.ResolveID(context.Background(), "#old")
	if err == nil {
		t.Fatal("expected error - #old not in new API response")
	}

	// But #new should work (fetched from API)
	client.index = 0 // Reset for another call
	id, err = resolver.ResolveID(context.Background(), "#new")
	if err != nil {
		t.Fatalf("ResolveID #new returned error: %v", err)
	}
	if id != "C4" {
		t.Fatalf("expected C4, got %s", id)
	}
}
