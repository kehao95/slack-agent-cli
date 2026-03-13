package channels

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/cache"
	"github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/slack"
)

var conversationIDPattern = regexp.MustCompile(`^[CDG][A-Z0-9]+$`)

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

	// Support Slack message permalinks like:
	// https://workspace.slack.com/archives/C123/p1705312365000100
	// https://workspace.slack.com/archives/D123/p1705312365000100
	if fromPermalink, ok := channelIDFromPermalink(trimmed); ok {
		return fromPermalink, nil
	}

	// If it looks like a channel ID, return as-is
	if isConversationID(trimmed) && !strings.Contains(trimmed, "#") {
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

	return "", errors.ChannelNotFoundError(trimmed)
}

// ResolveName returns the channel name for a given channel ID.
// Returns the ID itself if the name cannot be resolved.
func (r *Resolver) ResolveName(ctx context.Context, channelID string) string {
	// Load channels from cache
	channels, cursor, err := r.loadChannels(ctx)
	if err != nil {
		return channelID // Fallback to ID on error
	}

	// Search in cached channels
	for _, ch := range channels {
		if ch.ID == channelID {
			return ch.Name
		}
	}

	if r.client != nil {
		name := r.lookupNameByID(ctx, channelID, channels, cursor)
		if name != "" {
			return name
		}
	}

	// Not found in cache - try to fetch more if we have a client and cursor
	if r.client != nil && (cursor != "" || len(channels) == 0) {
		name := r.fetchNameForID(ctx, channelID, channels, cursor)
		if name != "" {
			return name
		}
	}

	return channelID // Fallback to ID if not found
}

func (r *Resolver) lookupNameByID(ctx context.Context, channelID string, channels []slackapi.Channel, cursor string) string {
	info, err := r.client.GetConversationInfo(ctx, channelID)
	if err != nil || info == nil {
		return ""
	}

	name := strings.TrimSpace(info.Name)
	if name == "" {
		return ""
	}

	r.cacheConversationInfo(channels, cursor, *info)
	return name
}

func (r *Resolver) cacheConversationInfo(channels []slackapi.Channel, cursor string, channel slackapi.Channel) {
	if r.cache == nil {
		return
	}

	for _, existing := range channels {
		if existing.ID == channel.ID {
			return
		}
	}

	updated := append(append([]slackapi.Channel{}, channels...), channel)
	if cursor != "" {
		_ = r.cache.SavePartial(cache.CacheKeyChannels, updated, cursor, false, len(updated))
		return
	}

	if len(channels) > 0 {
		_ = r.cache.Save(cache.CacheKeyChannels, updated)
	}
}

// fetchNameForID continues fetching pages until the channel ID is found.
func (r *Resolver) fetchNameForID(ctx context.Context, channelID string, existing []slackapi.Channel, cursor string) string {
	channels := existing
	currentCursor := cursor

	for {
		// Fetch next page
		page, nextCursor, err := r.client.ListChannels(ctx, slack.ListChannelsParams{
			Limit:           200,
			Cursor:          currentCursor,
			IncludeArchived: false,
			Types:           []string{"public_channel"},
		})
		if err != nil {
			// Save progress before returning
			if r.cache != nil && len(channels) > 0 {
				_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, currentCursor, false, len(channels))
			}
			return ""
		}

		// Search in new page
		for _, ch := range page {
			if ch.ID == channelID {
				// Found! Save progress and return
				channels = append(channels, page...)
				if r.cache != nil {
					if nextCursor == "" {
						_ = r.cache.PromotePartial(cache.CacheKeyChannels, channels)
					} else {
						_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, nextCursor, false, len(channels))
					}
				}
				return ch.Name
			}
		}

		channels = append(channels, page...)

		// No more pages
		if nextCursor == "" {
			// Save as complete
			if r.cache != nil {
				_ = r.cache.PromotePartial(cache.CacheKeyChannels, channels)
			}
			return "" // Not found
		}

		// Save progress and continue
		if r.cache != nil {
			_ = r.cache.SavePartial(cache.CacheKeyChannels, channels, nextCursor, false, len(channels))
		}
		currentCursor = nextCursor
	}
}

func isConversationID(input string) bool {
	return conversationIDPattern.MatchString(strings.TrimSpace(input))
}

func channelIDFromPermalink(input string) (string, bool) {
	u, err := url.Parse(input)
	if err != nil || u.Host == "" {
		return "", false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "archives" {
		return "", false
	}

	channelID := strings.ToUpper(parts[1])
	if !isConversationID(channelID) {
		return "", false
	}
	return channelID, true
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
