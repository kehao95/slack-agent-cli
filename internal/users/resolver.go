// Package users provides cached user profile lookups.
package users

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/cache"
	"github.com/kehao95/slack-agent-cli/internal/errors"
)

// UserClient defines the Slack operations needed for user lookups.
type UserClient interface {
	GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error)
	ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error)
	GetUserPresence(ctx context.Context, userID string) (*slackapi.UserPresence, error)
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

// RefreshCache clears the user cache.
func (r *Resolver) RefreshCache(ctx context.Context) error {
	if r.cache != nil {
		if err := r.cache.Expire(cache.CacheKeyUsers); err != nil {
			return err
		}
		if err := r.cache.ExpirePartial(cache.CacheKeyUsers); err != nil {
			return err
		}
	}
	return nil
}

// GetDisplayName returns a human-friendly name for a user ID.
// If the cache is empty, it will fetch all users from the API first.
func (r *Resolver) GetDisplayName(ctx context.Context, userID string) string {
	users, err := r.loadOrFetchUsers(ctx)
	if err != nil || users == nil {
		// Fallback to single lookup if bulk fetch failed
		if r.client != nil {
			info, err := r.client.GetUserInfo(ctx, userID)
			if err == nil {
				return displayName(toCachedUser(info))
			}
		}
		return userID
	}

	if u, ok := users[userID]; ok {
		return displayName(u)
	}

	// Not in cache, try single lookup and add to cache
	if r.client != nil {
		info, err := r.client.GetUserInfo(ctx, userID)
		if err == nil {
			cu := toCachedUser(info)
			users[userID] = cu
			// Update cache with new user
			if r.cache != nil {
				_ = r.cache.Save(cache.CacheKeyUsers, users)
			}
			return displayName(cu)
		}
	}

	return userID
}

// GetUser returns cached user info or fetches it.
func (r *Resolver) GetUser(ctx context.Context, userID string) (CachedUser, error) {
	users, err := r.loadOrFetchUsers(ctx)
	if err != nil {
		return CachedUser{}, err
	}
	if users == nil {
		users = make(map[string]CachedUser)
	}
	if u, ok := users[userID]; ok {
		return u, nil
	}
	if r.client == nil {
		return CachedUser{}, errors.UserNotFoundError(userID)
	}
	info, err := r.client.GetUserInfo(ctx, userID)
	if err != nil {
		return CachedUser{}, fmt.Errorf("get user %s: %w", userID, err)
	}
	cu := toCachedUser(info)
	users[userID] = cu
	if r.cache != nil {
		_ = r.cache.Save(cache.CacheKeyUsers, users)
	}
	return cu, nil
}

// loadOrFetchUsers returns the cached user map, fetching all users if cache is empty.
func (r *Resolver) loadOrFetchUsers(ctx context.Context) (map[string]CachedUser, error) {
	// Try to load from cache first
	users, err := r.loadUsers(ctx)
	if err != nil {
		return nil, err
	}
	if users != nil {
		return users, nil
	}

	// Cache is empty - fetch all users
	if r.client == nil {
		return nil, nil
	}

	allUsers, err := r.fetchAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to map and cache
	users = make(map[string]CachedUser, len(allUsers))
	for _, u := range allUsers {
		users[u.ID] = toCachedUser(&u)
	}

	if r.cache != nil {
		_ = r.cache.Save(cache.CacheKeyUsers, users)
	}

	return users, nil
}

// loadUsers returns the cached user map (from complete or partial cache).
// Does NOT auto-fetch from API.
func (r *Resolver) loadUsers(ctx context.Context) (map[string]CachedUser, error) {
	if r.cache == nil {
		return nil, nil
	}

	// Try complete cache first
	var cached map[string]CachedUser
	found, err := r.cache.Load(cache.CacheKeyUsers, &cached)
	if err != nil {
		return nil, err
	}
	if found && cached != nil {
		return cached, nil
	}

	// Try partial cache - convert from slice to map
	var partialUsers []slackapi.User
	state, found, err := r.cache.LoadPartial(cache.CacheKeyUsers, &partialUsers)
	if err != nil {
		return nil, err
	}
	if found && !state.Complete {
		users := make(map[string]CachedUser, len(partialUsers))
		for _, u := range partialUsers {
			users[u.ID] = toCachedUser(&u)
		}
		return users, nil
	}

	return nil, nil
}

// fetchAllUsers fetches all users from the API.
func (r *Resolver) fetchAllUsers(ctx context.Context) ([]slackapi.User, error) {
	var all []slackapi.User
	cursor := ""
	for {
		users, nextCursor, err := r.client.ListUsers(ctx, cursor, 200)
		if err != nil {
			return all, err // Return what we have so far
		}
		all = append(all, users...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return all, nil
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

func displayName(u CachedUser) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	return u.Name
}
