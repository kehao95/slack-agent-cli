package channels

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/slack"
)

// ErrCacheIncomplete is returned when the cache doesn't have enough data to resolve a channel.
type ErrCacheIncomplete struct {
	CachedCount int
	ChannelName string
}

func (e ErrCacheIncomplete) Error() string {
	return fmt.Sprintf("channel %q not found in cache (%d channels cached). "+
		"Run: slack-cli cache populate channels --all", e.ChannelName, e.CachedCount)
}

// Resolver resolves channel names to IDs using disk-cached lookups.
type Resolver struct {
	client slack.ChannelClient
	cache  *cache.Store
}

// NewResolver creates a Resolver with no cache (API-only).
func NewResolver(client slack.ChannelClient) *Resolver {
	return &Resolver{client: client}
}

// NewCachedResolver creates a Resolver backed by the given cache store.
func NewCachedResolver(client slack.ChannelClient, store *cache.Store) *Resolver {
	return &Resolver{client: client, cache: store}
}

// RefreshCache forces a cache refresh for channels by clearing existing cache.
// Use "slack-cli cache populate channels --all" to repopulate.
func (r *Resolver) RefreshCache(ctx context.Context) error {
	if r.cache != nil {
		if err := r.cache.Expire(cache.CacheKeyChannels); err != nil {
			return err
		}
		if err := r.cache.ExpirePartial(cache.CacheKeyChannels); err != nil {
			return err
		}
	}
	return nil
}

// ResolveID returns a channel ID for a provided name or ID string.
// If the cache is incomplete and the channel is not found, returns ErrCacheIncomplete.
func (r *Resolver) ResolveID(ctx context.Context, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("channel is required")
	}
	// If it looks like a channel ID, return as-is
	if strings.HasPrefix(trimmed, "C") && !strings.Contains(trimmed, "#") {
		return trimmed, nil
	}
	normalized := strings.TrimPrefix(trimmed, "#")

	channels, complete, err := r.loadChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve channel %s: %w", trimmed, err)
	}

	for _, ch := range channels {
		if strings.EqualFold(ch.Name, normalized) {
			return ch.ID, nil
		}
	}

	// Not found - provide helpful error based on cache state
	if !complete {
		return "", ErrCacheIncomplete{CachedCount: len(channels), ChannelName: trimmed}
	}
	return "", fmt.Errorf("channel %s not found", trimmed)
}

// loadChannels returns the cached channel list and whether the cache is complete.
// It checks both complete and partial caches. Does NOT auto-fetch from API.
func (r *Resolver) loadChannels(ctx context.Context) ([]slackapi.Channel, bool, error) {
	if r.cache == nil {
		// No cache configured - fall back to direct API fetch (legacy behavior)
		channels, err := r.fetchAllChannels(ctx)
		return channels, true, err
	}

	// Try complete cache first
	var cached []slackapi.Channel
	found, err := r.cache.Load(cache.CacheKeyChannels, &cached)
	if err != nil {
		return nil, false, err
	}
	if found {
		return cached, true, nil
	}

	// Try partial cache
	var partial []slackapi.Channel
	state, found, err := r.cache.LoadPartial(cache.CacheKeyChannels, &partial)
	if err != nil {
		return nil, false, err
	}
	if found {
		return partial, state.Complete, nil
	}

	// No cache at all - return empty with hint to populate
	return nil, false, nil
}

// fetchAllChannels fetches all channels from the API (legacy fallback for non-cached resolver).
func (r *Resolver) fetchAllChannels(ctx context.Context) ([]slackapi.Channel, error) {
	var all []slackapi.Channel
	params := slack.ListChannelsParams{Limit: 1000, IncludeArchived: true, Types: defaultChannelTypes}
	for {
		var channels []slackapi.Channel
		var cursor string
		err := slack.DoWithRetry(ctx, func() error {
			var err error
			channels, cursor, err = r.client.ListChannels(ctx, params)
			return err
		})
		if err != nil {
			return nil, err
		}
		all = append(all, channels...)
		if cursor == "" {
			break
		}
		params.Cursor = cursor
	}
	return all, nil
}
