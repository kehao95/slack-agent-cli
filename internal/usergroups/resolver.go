// Package usergroups provides cached usergroup lookups.
package usergroups

import (
	"context"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
)

// UserGroupClient defines the Slack operations needed for usergroup lookups.
type UserGroupClient interface {
	GetUserGroups(ctx context.Context) ([]slackapi.UserGroup, error)
}

// CachedUserGroup holds the subset of usergroup info we persist.
type CachedUserGroup struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

// Resolver resolves usergroup IDs to names using a disk cache.
type Resolver struct {
	client UserGroupClient
	cache  *cache.Store
}

// NewResolver creates a Resolver with no cache (API-only).
func NewResolver(client UserGroupClient) *Resolver {
	return &Resolver{client: client}
}

// NewCachedResolver creates a Resolver backed by the given cache store.
func NewCachedResolver(client UserGroupClient, store *cache.Store) *Resolver {
	return &Resolver{client: client, cache: store}
}

// RefreshCache clears the usergroup cache.
func (r *Resolver) RefreshCache(ctx context.Context) error {
	if r.cache != nil {
		if err := r.cache.Expire(cache.CacheKeyUserGroups); err != nil {
			return err
		}
	}
	return nil
}

// GetHandle returns the usergroup handle (e.g., "pf-devx") for a usergroup ID.
// If the cache is empty, it will fetch all usergroups from the API first.
func (r *Resolver) GetHandle(ctx context.Context, groupID string) string {
	groups, err := r.loadOrFetchUserGroups(ctx)
	if err != nil || groups == nil {
		return groupID
	}

	if g, ok := groups[groupID]; ok {
		return g.Handle
	}

	return groupID
}

// loadOrFetchUserGroups returns the cached usergroup map, fetching all usergroups if cache is empty.
func (r *Resolver) loadOrFetchUserGroups(ctx context.Context) (map[string]CachedUserGroup, error) {
	// Try to load from cache first
	groups, err := r.loadUserGroups(ctx)
	if err != nil {
		return nil, err
	}
	if groups != nil {
		return groups, nil
	}

	// Cache is empty - fetch all usergroups
	if r.client == nil {
		return nil, nil
	}

	allGroups, err := r.client.GetUserGroups(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to map and cache
	groups = make(map[string]CachedUserGroup, len(allGroups))
	for _, g := range allGroups {
		groups[g.ID] = toCachedUserGroup(&g)
	}

	if r.cache != nil {
		_ = r.cache.Save(cache.CacheKeyUserGroups, groups)
	}

	return groups, nil
}

// loadUserGroups returns the cached usergroup map from disk.
func (r *Resolver) loadUserGroups(ctx context.Context) (map[string]CachedUserGroup, error) {
	if r.cache == nil {
		return nil, nil
	}

	var cached map[string]CachedUserGroup
	found, err := r.cache.Load(cache.CacheKeyUserGroups, &cached)
	if err != nil {
		return nil, err
	}
	if found && cached != nil {
		return cached, nil
	}

	return nil, nil
}

func toCachedUserGroup(g *slackapi.UserGroup) CachedUserGroup {
	return CachedUserGroup{
		ID:     g.ID,
		Name:   g.Name,
		Handle: g.Handle,
	}
}
