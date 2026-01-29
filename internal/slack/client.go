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
