package channels

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/slack"
)

// CacheKeyChannels is the cache key for the full channel list.
const CacheKeyChannels = "channels"

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

// RefreshCache forces a cache refresh for channels.
func (r *Resolver) RefreshCache(ctx context.Context) error {
	if r.cache != nil {
		if err := r.cache.Expire(CacheKeyChannels); err != nil {
			return err
		}
	}
	// Fetch and cache
	_, err := r.loadChannels(ctx)
	return err
}

// ResolveID returns a channel ID for a provided name or ID string.
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

	channels, err := r.loadChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve channel %s: %w", trimmed, err)
	}

	for _, ch := range channels {
		if strings.EqualFold(ch.Name, normalized) {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("channel %s not found", trimmed)
}

// loadChannels returns the cached channel list or fetches from the API.
func (r *Resolver) loadChannels(ctx context.Context) ([]slackapi.Channel, error) {
	// Try cache first
	if r.cache != nil {
		var cached []slackapi.Channel
		found, err := r.cache.Load(CacheKeyChannels, &cached)
		if err != nil {
			return nil, err
		}
		if found {
			return cached, nil
		}
	}

	// Fetch all channels from Slack API
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

	// Persist to cache
	if r.cache != nil {
		if err := r.cache.Save(CacheKeyChannels, all); err != nil {
			// Log but don't fail—cache is optimization only
			_ = err
		}
	}

	return all, nil
}
