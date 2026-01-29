// Package users provides cached user profile lookups.
package users

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
)

// CacheKeyUsers is the cache key for the user map.
const CacheKeyUsers = "users"

// UserClient defines the Slack operations needed for user lookups.
type UserClient interface {
	GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error)
	GetUsers(ctx context.Context) ([]slackapi.User, error)
}

// CachedUser holds the subset of user info we persist.
type CachedUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	IsBot       bool   `json:"is_bot"`
}

// Resolver resolves user IDs to display names using a disk cache.
type Resolver struct {
	client UserClient
	cache  *cache.Store
}

// NewResolver creates a Resolver with no cache (API-only).
func NewResolver(client UserClient) *Resolver {
	return &Resolver{client: client}
}

// NewCachedResolver creates a Resolver backed by the given cache store.
func NewCachedResolver(client UserClient, store *cache.Store) *Resolver {
	return &Resolver{client: client, cache: store}
}

// RefreshCache forces a full user list refresh.
func (r *Resolver) RefreshCache(ctx context.Context) error {
	if r.cache != nil {
		if err := r.cache.Expire(CacheKeyUsers); err != nil {
			return err
		}
	}
	_, err := r.loadUsers(ctx)
	return err
}

// GetDisplayName returns a human-friendly name for a user ID.
// Falls back to the raw user ID if lookup fails.
func (r *Resolver) GetDisplayName(ctx context.Context, userID string) string {
	users, err := r.loadUsers(ctx)
	if err != nil {
		return userID
	}
	if u, ok := users[userID]; ok {
		if u.DisplayName != "" {
			return u.DisplayName
		}
		if u.RealName != "" {
			return u.RealName
		}
		return u.Name
	}
	// Not in cache, try single lookup
	info, err := r.client.GetUserInfo(ctx, userID)
	if err != nil {
		return userID
	}
	cu := toCachedUser(info)
	// Update cache
	users[userID] = cu
	if r.cache != nil {
		_ = r.cache.Save(CacheKeyUsers, users)
	}
	if cu.DisplayName != "" {
		return cu.DisplayName
	}
	if cu.RealName != "" {
		return cu.RealName
	}
	return cu.Name
}

// GetUser returns cached user info or fetches it.
func (r *Resolver) GetUser(ctx context.Context, userID string) (CachedUser, error) {
	users, err := r.loadUsers(ctx)
	if err != nil {
		return CachedUser{}, err
	}
	if u, ok := users[userID]; ok {
		return u, nil
	}
	info, err := r.client.GetUserInfo(ctx, userID)
	if err != nil {
		return CachedUser{}, fmt.Errorf("get user %s: %w", userID, err)
	}
	cu := toCachedUser(info)
	users[userID] = cu
	if r.cache != nil {
		_ = r.cache.Save(CacheKeyUsers, users)
	}
	return cu, nil
}

// loadUsers returns the cached user map or fetches all users from the API.
func (r *Resolver) loadUsers(ctx context.Context) (map[string]CachedUser, error) {
	if r.cache != nil {
		var cached map[string]CachedUser
		found, err := r.cache.Load(CacheKeyUsers, &cached)
		if err != nil {
			return nil, err
		}
		if found && cached != nil {
			return cached, nil
		}
	}

	// Fetch all users
	list, err := r.client.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make(map[string]CachedUser, len(list))
	for _, u := range list {
		users[u.ID] = toCachedUser(&u)
	}

	if r.cache != nil {
		_ = r.cache.Save(CacheKeyUsers, users)
	}
	return users, nil
}

func toCachedUser(u *slackapi.User) CachedUser {
	return CachedUser{
		ID:          u.ID,
		Name:        u.Name,
		RealName:    u.RealName,
		DisplayName: u.Profile.DisplayName,
		IsBot:       u.IsBot,
	}
}
