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

func TestResolverResolveID(t *testing.T) {
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

	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "cached", Conversation: slackapi.Conversation{ID: "C2"}}}},
		},
	}

	resolver := NewCachedResolver(client, store)

	// First call fetches from API
	id, err := resolver.ResolveID(context.Background(), "#cached")
	if err != nil {
		t.Fatalf("first ResolveID returned error: %v", err)
	}
	if id != "C2" {
		t.Fatalf("expected C2, got %s", id)
	}
	if client.index != 1 {
		t.Fatalf("expected one API call, got %d", client.index)
	}

	// Second call should hit cache, no additional API call
	id2, err := resolver.ResolveID(context.Background(), "#cached")
	if err != nil {
		t.Fatalf("second ResolveID returned error: %v", err)
	}
	if id2 != "C2" {
		t.Fatalf("expected C2 from cache, got %s", id2)
	}
	if client.index != 1 {
		t.Fatalf("expected no additional API call, got %d", client.index)
	}
}

func TestResolverRefreshCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	client := &resolverMockClient{
		responses: [][]slackapi.Channel{
			{{GroupConversation: slackapi.GroupConversation{Name: "old", Conversation: slackapi.Conversation{ID: "C3"}}}},
			{{GroupConversation: slackapi.GroupConversation{Name: "new", Conversation: slackapi.Conversation{ID: "C4"}}}},
		},
	}

	resolver := NewCachedResolver(client, store)

	// First call
	_, _ = resolver.ResolveID(context.Background(), "#old")
	if client.index != 1 {
		t.Fatalf("expected one API call, got %d", client.index)
	}

	// Force refresh
	if err := resolver.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache returned error: %v", err)
	}
	if client.index != 2 {
		t.Fatalf("expected two API calls after refresh, got %d", client.index)
	}

	// Now #new should resolve
	id, err := resolver.ResolveID(context.Background(), "#new")
	if err != nil {
		t.Fatalf("ResolveID #new returned error: %v", err)
	}
	if id != "C4" {
		t.Fatalf("expected C4, got %s", id)
	}
}
