package channels

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/slack"
)

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
// Use "slack-cli cache populate channels" to repopulate.
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
// If the channel is not found in cache, it will fetch more pages from the API.
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

	// First, check existing cache
	channels, cursor, err := r.loadChannels(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve channel %s: %w", trimmed, err)
	}

	// Search in cached channels
	for _, ch := range channels {
		if strings.EqualFold(ch.Name, normalized) {
			return ch.ID, nil
		}
	}

	// Not found in cache - fetch from API if we have a client
	// Fetch if: we have more pages (cursor != "") OR we have no cached data yet
	if r.client != nil && (cursor != "" || len(channels) == 0) {
		id, err := r.fetchUntilFound(ctx, normalized, channels, cursor)
		if err != nil {
			return "", fmt.Errorf("resolve channel %s: %w", trimmed, err)
		}
		if id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("channel %s not found", trimmed)
}

// loadChannels returns the cached channel list and the next cursor (if partial).
func (r *Resolver) loadChannels(ctx context.Context) ([]slackapi.Channel, string, error) {
	if r.cache == nil {
		// No cache configured - return empty, will fetch on demand
		return nil, "", nil
	}

	// Try complete cache first
	var cached []slackapi.Channel
	found, err := r.cache.Load(cache.CacheKeyChannels, &cached)
	if err != nil {
		return nil, "", err
	}
	if found {
		return cached, "", nil // Empty cursor means complete
	}

	// Try partial cache
	var partial []slackapi.Channel
	state, found, err := r.cache.LoadPartial(cache.CacheKeyChannels, &partial)
	if err != nil {
		return nil, "", err
	}
	if found {
		return partial, state.NextCursor, nil
	}

	// No cache at all - return empty with empty cursor (will fetch from start)
	return nil, "", nil
}

// fetchUntilFound continues fetching pages until the channel is found or no more pages.
// Updates the cache as it fetches.
func (r *Resolver) fetchUntilFound(ctx context.Context, name string, existing []slackapi.Channel, cursor string) (string, error) {
	channels := existing
	currentCursor := cursor

	// If no cursor, start from beginning
	if currentCursor == "" && len(channels) == 0 {
		currentCursor = "" // Will fetch first page
	}

	for {
		// Fetch next page
		// Note: Only fetch public_channel to avoid scope issues (private_channel requires groups:read)
		page, nextCursor, err := r.client.ListChannels(ctx, slack.ListChannelsParams{
			Limit:           200,
			Cursor:          currentCursor,
			IncludeArchived: false,
			Types:           []string{"public_channel"},
		})
		if err != nil {
			// Save progress before returning error
			if r.cache != nil && len(channels) > 0 {
				_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, currentCursor, false, len(channels))
			}
			return "", err
		}

		// Search in new page
		for _, ch := range page {
			if strings.EqualFold(ch.Name, name) {
				// Found! Save progress and return
				channels = append(channels, page...)
				if r.cache != nil {
					if nextCursor == "" {
						_ = r.cache.PromotePartial(cache.CacheKeyChannels, channels)
					} else {
						_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, nextCursor, false, len(channels))
					}
				}
				return ch.ID, nil
			}
		}

		channels = append(channels, page...)

		// No more pages
		if nextCursor == "" {
			// Save as complete
			if r.cache != nil {
				_ = r.cache.PromotePartial(cache.CacheKeyChannels, channels)
			}
			return "", nil // Not found
		}

		// Save progress and continue
		if r.cache != nil {
			_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, nextCursor, false, len(channels))
		}
		currentCursor = nextCursor
	}
}
