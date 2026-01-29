package slack

import (
	"context"
	"fmt"
	"time"

	slackapi "github.com/slack-go/slack"
)

// Client defines the subset of Slack operations used by the CLI.
type Client interface {
	ListConversationsHistory(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error)
	ListThreadReplies(ctx context.Context, params ThreadParams) ([]slackapi.Message, bool, string, error)
}

// ChannelClient extends Client with channel operations.
type ChannelClient interface {
	Client
	ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error)
}

// UserClient extends Client with user operations.
type UserClient interface {
	Client
	GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error)
	ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error)
}

// FullClient combines all client capabilities.
type FullClient interface {
	ChannelClient
	UserClient
}

// HistoryParams wraps the arguments to conversations.history.
type HistoryParams struct {
	Channel   string
	Cursor    string
	Limit     int
	Latest    string
	Oldest    string
	Inclusive bool
}

// ThreadParams wraps arguments for conversations.replies.
type ThreadParams struct {
	Channel string
	Cursor  string
	Limit   int
	Latest  string
	Oldest  string
	Thread  string
}

// APIClient implements Client by wrapping slack-go's Client.
type APIClient struct {
	sdk *slackapi.Client
}

// New creates a new APIClient using the provided bot token.
func New(botToken string) *APIClient {
	return &APIClient{sdk: slackapi.New(botToken)}
}

// ListConversationsHistory retrieves channel history.
func (c *APIClient) ListConversationsHistory(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error) {
	if params.Channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	options := &slackapi.GetConversationHistoryParameters{ChannelID: params.Channel}
	options.Cursor = params.Cursor
	options.Limit = params.Limit
	options.Latest = params.Latest
	options.Oldest = params.Oldest
	options.Inclusive = params.Inclusive

	return c.sdk.GetConversationHistoryContext(ctx, options)
}

// ListThreadReplies fetches messages in a thread.
func (c *APIClient) ListThreadReplies(ctx context.Context, params ThreadParams) ([]slackapi.Message, bool, string, error) {
	if params.Channel == "" || params.Thread == "" {
		return nil, false, "", fmt.Errorf("channel and thread are required")
	}
	opts := &slackapi.GetConversationRepliesParameters{ChannelID: params.Channel, Timestamp: params.Thread}
	opts.Cursor = params.Cursor
	opts.Limit = params.Limit
	opts.Latest = params.Latest
	opts.Oldest = params.Oldest
	msgs, hasMore, nextCursor, err := c.sdk.GetConversationRepliesContext(ctx, opts)
	return msgs, hasMore, nextCursor, err
}

// ListChannels fetches channel metadata.
func (c *APIClient) ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error) {
	convParams := &slackapi.GetConversationsParameters{
		Limit:  params.Limit,
		Cursor: params.Cursor,
	}
	convParams.Types = append(convParams.Types, params.Types...)
	if !params.IncludeArchived {
		convParams.ExcludeArchived = true
	}
	channels, nextCursor, err := c.sdk.GetConversationsContext(ctx, convParams)
	return channels, nextCursor, err
}

// ListChannelsParams controls ListChannels behavior.
type ListChannelsParams struct {
	Limit           int
	Cursor          string
	IncludeArchived bool
	Types           []string
}

// GetUserInfo fetches a single user's info.
func (c *APIClient) GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error) {
	user, err := c.sdk.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	return user, nil
}

// ListUsers fetches users with pagination using slack-go's pagination API.
// Note: slack-go doesn't expose cursor directly, so we fetch one page at a time
// using GetUsers with limit. The cursor parameter is ignored for now.
func (c *APIClient) ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error) {
	// slack-go's GetUsers doesn't support cursor-based pagination in the same way.
	// We use GetUsersPaginated iterator but fetch one page at a time.
	// For simplicity, fetch all users in one call (the SDK handles pagination internally).
	// This is a limitation - for very large workspaces, consider using the raw API.
	users, err := c.sdk.GetUsersContext(ctx, slackapi.GetUsersOptionLimit(limit))
	if err != nil {
		return nil, "", fmt.Errorf("list users: %w", err)
	}
	// Return empty cursor since we fetched all
	return users, "", nil
}

// ListChannelsPaginated provides a simpler interface for cache population.
// Returns all visible channels (both member and non-member).
func (c *APIClient) ListChannelsPaginated(ctx context.Context, cursor string, limit int) ([]slackapi.Channel, string, int, error) {
	channels, nextCursor, err := c.ListChannels(ctx, ListChannelsParams{
		Limit:           limit,
		Cursor:          cursor,
		IncludeArchived: false,
		Types:           []string{"public_channel", "private_channel"},
	})
	if err != nil {
		return nil, "", 0, err
	}

	return channels, nextCursor, len(channels), nil
}

// DoWithRetry executes fn with simple retry logic for rate-limited operations.
func DoWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	backoff := time.Second
	for attempts := 0; attempts < 3; attempts++ {
		if err := fn(); err != nil {
			if rlErr, ok := err.(*slackapi.RateLimitedError); ok {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(rlErr.RetryAfter):
					lastErr = err
					continue
				}
			}
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return nil
	}
	return lastErr
}
