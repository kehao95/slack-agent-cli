// Package users provides cached user profile lookups.
package users

import (
	"context"
	"fmt"
	"strings"

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

const cacheKeyUserPrefix = "user_"

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
		if err := r.cache.ExpireAll(cacheKeyUserPrefix); err != nil {
			return err
		}
	}
	return nil
}

// GetDisplayName returns a human-friendly name for a user ID.
func (r *Resolver) GetDisplayName(ctx context.Context, userID string) string {
	if u, found, err := r.loadSingleUser(userID); err == nil && found {
		if name := displayName(u); name != "" && name != userID {
			return name
		}
	}

	cu, err := r.fetchAndCacheUser(ctx, userID)
	if err == nil {
		return displayName(cu)
	}

	return userID
}

// GetMentionName returns a handle-like value suitable for @-style references.
func (r *Resolver) GetMentionName(ctx context.Context, userID string) string {
	if u, found, err := r.loadSingleUser(userID); err == nil && found {
		if name := mentionName(u); name != "" && name != userID {
			return name
		}
	}

	cu, err := r.fetchAndCacheUser(ctx, userID)
	if err == nil {
		return mentionName(cu)
	}

	return userID
}

// GetUser returns cached user info or fetches it.
func (r *Resolver) GetUser(ctx context.Context, userID string) (CachedUser, error) {
	if u, found, err := r.loadSingleUser(userID); err == nil && found {
		return u, nil
	}

	cu, err := r.fetchAndCacheUser(ctx, userID)
	if err != nil {
		return CachedUser{}, err
	}
	return cu, nil
}

func (r *Resolver) fetchAndCacheUser(ctx context.Context, userID string) (CachedUser, error) {
	if r.client == nil {
		return CachedUser{}, errors.UserNotFoundError(userID)
	}

	info, err := r.client.GetUserInfo(ctx, userID)
	if err != nil {
		return CachedUser{}, fmt.Errorf("get user %s: %w", userID, err)
	}

	cu := toCachedUser(info)
	r.cacheSingleUser(userID, cu)
	return cu, nil
}

func (r *Resolver) cacheSingleUser(userID string, cu CachedUser) {
	if r.cache != nil {
		_ = r.cache.Save(userCacheKey(userID), cu)
	}
}

func (r *Resolver) loadSingleUser(userID string) (CachedUser, bool, error) {
	if r.cache == nil {
		return CachedUser{}, false, nil
	}

	var cached CachedUser
	found, err := r.cache.Load(userCacheKey(userID), &cached)
	if err != nil {
		return CachedUser{}, false, err
	}
	if found && cached.ID != "" {
		return cached, true, nil
	}
	return CachedUser{}, false, nil
}

func userCacheKey(userID string) string {
	return cacheKeyUserPrefix + strings.TrimSpace(userID)
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

func mentionName(u CachedUser) string {
	if u.Name != "" {
		return u.Name
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	return u.ID
}
