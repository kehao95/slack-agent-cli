package users

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
)

type mockUserClient struct {
	users       []slackapi.User
	singleUser  *slackapi.User
	err         error
	callsGetAll int
	callsGetOne int
}

func (m *mockUserClient) GetUsers(ctx context.Context) ([]slackapi.User, error) {
	m.callsGetAll++
	if m.err != nil {
		return nil, m.err
	}
	return m.users, nil
}

func (m *mockUserClient) GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error) {
	m.callsGetOne++
	if m.err != nil {
		return nil, m.err
	}
	if m.singleUser != nil {
		return m.singleUser, nil
	}
	for i := range m.users {
		if m.users[i].ID == userID {
			return &m.users[i], nil
		}
	}
	return nil, errors.New("user not found")
}

func TestResolver_GetDisplayName(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	client := &mockUserClient{
		users: []slackapi.User{
			{ID: "U1", Name: "alice", RealName: "Alice Smith", Profile: slackapi.UserProfile{DisplayName: "Alice"}},
		},
	}

	resolver := NewCachedResolver(client, store)

	name := resolver.GetDisplayName(context.Background(), "U1")
	if name != "Alice" {
		t.Errorf("expected Alice, got %s", name)
	}
	if client.callsGetAll != 1 {
		t.Errorf("expected 1 GetUsers call, got %d", client.callsGetAll)
	}

	// Second call should hit cache
	name2 := resolver.GetDisplayName(context.Background(), "U1")
	if name2 != "Alice" {
		t.Errorf("expected Alice on cache hit, got %s", name2)
	}
	if client.callsGetAll != 1 {
		t.Errorf("expected no additional GetUsers call, got %d", client.callsGetAll)
	}
}

func TestResolver_GetDisplayName_Unknown(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	client := &mockUserClient{
		users:      []slackapi.User{},
		singleUser: &slackapi.User{ID: "U2", Name: "bob", RealName: "Bob Jones", Profile: slackapi.UserProfile{DisplayName: "Bobby"}},
	}

	resolver := NewCachedResolver(client, store)

	name := resolver.GetDisplayName(context.Background(), "U2")
	if name != "Bobby" {
		t.Errorf("expected Bobby, got %s", name)
	}
	if client.callsGetOne != 1 {
		t.Errorf("expected 1 GetUserInfo call, got %d", client.callsGetOne)
	}
}

func TestResolver_RefreshCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	client := &mockUserClient{
		users: []slackapi.User{
			{ID: "U3", Name: "charlie", RealName: "Charlie", Profile: slackapi.UserProfile{DisplayName: "Chuck"}},
		},
	}

	resolver := NewCachedResolver(client, store)

	_ = resolver.GetDisplayName(context.Background(), "U3")
	if client.callsGetAll != 1 {
		t.Errorf("expected 1 call, got %d", client.callsGetAll)
	}

	// Refresh and verify new call
	if err := resolver.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache error: %v", err)
	}
	if client.callsGetAll != 2 {
		t.Errorf("expected 2 calls after refresh, got %d", client.callsGetAll)
	}
}
