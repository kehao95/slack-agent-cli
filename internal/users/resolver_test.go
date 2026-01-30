package users

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/cache"
)

type mockUserClient struct {
	singleUser   *slackapi.User
	users        map[string]*slackapi.User
	allUsers     []slackapi.User // For ListUsers
	err          error
	listUsersErr error
	callsGetOne  int
	callsListAll int
}

func (m *mockUserClient) GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error) {
	m.callsGetOne++
	if m.err != nil {
		return nil, m.err
	}
	if m.singleUser != nil && m.singleUser.ID == userID {
		return m.singleUser, nil
	}
	if m.users != nil {
		if u, ok := m.users[userID]; ok {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserClient) ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error) {
	m.callsListAll++
	if m.listUsersErr != nil {
		return nil, "", m.listUsersErr
	}
	// Return all users with empty cursor (simulating single page)
	return m.allUsers, "", nil
}

func TestResolver_GetDisplayName_FromCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate cache with user data
	users := map[string]CachedUser{
		"U1": {ID: "U1", Name: "alice", RealName: "Alice Smith", DisplayName: "Alice"},
	}
	if err := store.Save(cache.CacheKeyUsers, users); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &mockUserClient{}
	resolver := NewCachedResolver(client, store)

	// Should find user in cache without API call
	name := resolver.GetDisplayName(context.Background(), "U1")
	if name != "Alice" {
		t.Errorf("expected Alice, got %s", name)
	}
	if client.callsGetOne != 0 {
		t.Errorf("expected 0 GetUserInfo calls (cache hit), got %d", client.callsGetOne)
	}
}

func TestResolver_GetDisplayName_NotInCache_FallbackToAPI(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate cache with different user
	users := map[string]CachedUser{
		"U1": {ID: "U1", Name: "alice", RealName: "Alice Smith", DisplayName: "Alice"},
	}
	if err := store.Save(cache.CacheKeyUsers, users); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &mockUserClient{
		singleUser: &slackapi.User{ID: "U2", Name: "bob", RealName: "Bob Jones", Profile: slackapi.UserProfile{DisplayName: "Bobby"}},
	}

	resolver := NewCachedResolver(client, store)

	// U2 not in cache - should call API
	name := resolver.GetDisplayName(context.Background(), "U2")
	if name != "Bobby" {
		t.Errorf("expected Bobby, got %s", name)
	}
	if client.callsGetOne != 1 {
		t.Errorf("expected 1 GetUserInfo call (cache miss), got %d", client.callsGetOne)
	}

	// Second call for U2 should now hit cache (user was added to cache)
	name2 := resolver.GetDisplayName(context.Background(), "U2")
	if name2 != "Bobby" {
		t.Errorf("expected Bobby on second call, got %s", name2)
	}
	if client.callsGetOne != 1 {
		t.Errorf("expected no additional API call after caching, got %d", client.callsGetOne)
	}
}

func TestResolver_GetDisplayName_NoCache_APIOnly(t *testing.T) {
	client := &mockUserClient{
		singleUser: &slackapi.User{ID: "U3", Name: "charlie", RealName: "Charlie", Profile: slackapi.UserProfile{DisplayName: "Chuck"}},
	}

	// No cache store - API only resolver
	resolver := NewResolver(client)

	name := resolver.GetDisplayName(context.Background(), "U3")
	if name != "Chuck" {
		t.Errorf("expected Chuck, got %s", name)
	}
	if client.callsGetOne != 1 {
		t.Errorf("expected 1 GetUserInfo call, got %d", client.callsGetOne)
	}

	// Without cache, every call goes to API
	name2 := resolver.GetDisplayName(context.Background(), "U3")
	if name2 != "Chuck" {
		t.Errorf("expected Chuck on second call, got %s", name2)
	}
	if client.callsGetOne != 2 {
		t.Errorf("expected 2 GetUserInfo calls (no cache), got %d", client.callsGetOne)
	}
}

func TestResolver_GetDisplayName_UnknownUser(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Empty cache
	users := map[string]CachedUser{}
	if err := store.Save(cache.CacheKeyUsers, users); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &mockUserClient{
		err: errors.New("user_not_found"),
	}

	resolver := NewCachedResolver(client, store)

	// Unknown user should return raw ID
	name := resolver.GetDisplayName(context.Background(), "UUNKNOWN")
	if name != "UUNKNOWN" {
		t.Errorf("expected UUNKNOWN (fallback), got %s", name)
	}
}

func TestResolver_RefreshCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate cache
	users := map[string]CachedUser{
		"U1": {ID: "U1", Name: "alice", DisplayName: "Alice"},
	}
	if err := store.Save(cache.CacheKeyUsers, users); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &mockUserClient{}
	resolver := NewCachedResolver(client, store)

	// Verify cache is populated
	name := resolver.GetDisplayName(context.Background(), "U1")
	if name != "Alice" {
		t.Errorf("expected Alice before refresh, got %s", name)
	}

	// RefreshCache should clear the cache
	if err := resolver.RefreshCache(context.Background()); err != nil {
		t.Fatalf("RefreshCache error: %v", err)
	}

	// After refresh, cache is empty - U1 not found, fallback to API or return ID
	client.err = errors.New("user_not_found")
	name2 := resolver.GetDisplayName(context.Background(), "U1")
	if name2 != "U1" {
		t.Errorf("expected U1 (fallback after cache clear), got %s", name2)
	}
}

func TestResolver_GetDisplayName_PartialCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Simulate partial cache from populate command
	partialUsers := []slackapi.User{
		{ID: "U1", Name: "alice", RealName: "Alice Smith", Profile: slackapi.UserProfile{DisplayName: "Alice"}},
		{ID: "U2", Name: "bob", RealName: "Bob Jones", Profile: slackapi.UserProfile{DisplayName: "Bobby"}},
	}
	if err := store.SavePartial(cache.CacheKeyUsers, partialUsers, "next_page_cursor", false, len(partialUsers)); err != nil {
		t.Fatalf("failed to save partial cache: %v", err)
	}

	client := &mockUserClient{}
	resolver := NewCachedResolver(client, store)

	// Should find U1 in partial cache
	name := resolver.GetDisplayName(context.Background(), "U1")
	if name != "Alice" {
		t.Errorf("expected Alice from partial cache, got %s", name)
	}
	if client.callsGetOne != 0 {
		t.Errorf("expected 0 API calls for cached user, got %d", client.callsGetOne)
	}

	// Should find U2 in partial cache
	name2 := resolver.GetDisplayName(context.Background(), "U2")
	if name2 != "Bobby" {
		t.Errorf("expected Bobby from partial cache, got %s", name2)
	}
}

func TestResolver_GetUser(t *testing.T) {
	dir := t.TempDir()
	store := cache.New(dir, cache.DefaultTTL)

	// Pre-populate cache
	users := map[string]CachedUser{
		"U1": {ID: "U1", Name: "alice", RealName: "Alice Smith", DisplayName: "Alice", IsBot: false},
	}
	if err := store.Save(cache.CacheKeyUsers, users); err != nil {
		t.Fatalf("failed to pre-populate cache: %v", err)
	}

	client := &mockUserClient{
		singleUser: &slackapi.User{ID: "U2", Name: "bot", RealName: "Bot User", IsBot: true, Profile: slackapi.UserProfile{DisplayName: "BotDisplay"}},
	}
	resolver := NewCachedResolver(client, store)

	// Get cached user
	u1, err := resolver.GetUser(context.Background(), "U1")
	if err != nil {
		t.Fatalf("GetUser U1 error: %v", err)
	}
	if u1.Name != "alice" || u1.DisplayName != "Alice" {
		t.Errorf("unexpected user data: %+v", u1)
	}

	// Get user not in cache (triggers API)
	u2, err := resolver.GetUser(context.Background(), "U2")
	if err != nil {
		t.Fatalf("GetUser U2 error: %v", err)
	}
	if u2.Name != "bot" || !u2.IsBot {
		t.Errorf("unexpected user data: %+v", u2)
	}
	if client.callsGetOne != 1 {
		t.Errorf("expected 1 API call for uncached user, got %d", client.callsGetOne)
	}
}
