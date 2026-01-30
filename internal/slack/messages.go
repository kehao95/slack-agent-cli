package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
)

// MessageFetcher handles message retrieval logic.
type MessageFetcher struct {
	client Client
}

func NewMessageFetcher(client Client) *MessageFetcher {
	return &MessageFetcher{client: client}
}

// ListMessages fetches messages according to params.
func (mf *MessageFetcher) ListMessages(ctx context.Context, params HistoryParams) ([]slackapi.Message, string, bool, error) {
	resp, err := mf.client.ListConversationsHistory(ctx, params)
	if err != nil {
		return nil, "", false, fmt.Errorf("get conversation history: %w", err)
	}
	return resp.Messages, resp.ResponseMetaData.NextCursor, resp.HasMore, nil
}

// ListThread fetches a thread's messages.
func (mf *MessageFetcher) ListThread(ctx context.Context, params ThreadParams) ([]slackapi.Message, string, bool, error) {
	msgs, hasMore, cursor, err := mf.client.ListThreadReplies(ctx, params)
	if err != nil {
		return nil, "", false, fmt.Errorf("get thread replies: %w", err)
	}
	return msgs, cursor, hasMore, nil
}

// ParseTimeRange converts textual inputs into Slack-compatible timestamps.
func ParseTimeRange(since, until string) (string, string, error) {
	var oldest, latest string
	if since != "" {
		parsed, err := parseTimeInput(since)
		if err != nil {
			return "", "", fmt.Errorf("parse since: %w", err)
		}
		oldest = formatSlackTimestamp(parsed)
	}
	if until != "" {
		parsed, err := parseTimeInput(until)
		if err != nil {
			return "", "", fmt.Errorf("parse until: %w", err)
		}
		latest = formatSlackTimestamp(parsed)
	}
	return oldest, latest, nil
}

func parseTimeInput(value string) (time.Time, error) {
	switch {
	case strings.HasSuffix(value, "h"), strings.HasSuffix(value, "m"), strings.HasSuffix(value, "s"):
		dur, err := time.ParseDuration(value)
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(-dur), nil
	default:
		return time.Parse(time.RFC3339, value)
	}
}

func formatSlackTimestamp(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000)
}

// ListConversationsHistory retrieves channel history.
func (c *APIClient) ListConversationsHistory(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error) {
	if params.Channel == "" {
		return nil, ErrChannelRequired
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

// PostMessage sends a message to a channel.
func (c *APIClient) PostMessage(ctx context.Context, channel string, opts PostMessageOptions) (*PostMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if opts.Text == "" && len(opts.Blocks) == 0 {
		return nil, ErrTextRequired
	}

	msgOpts := []slackapi.MsgOption{
		slackapi.MsgOptionText(opts.Text, false),
	}

	if opts.ThreadTS != "" {
		msgOpts = append(msgOpts, slackapi.MsgOptionTS(opts.ThreadTS))
	}

	if len(opts.Blocks) > 0 {
		msgOpts = append(msgOpts, slackapi.MsgOptionBlocks(opts.Blocks...))
	}

	// Only add disable options if unfurl is explicitly false
	if !opts.UnfurlLinks {
		msgOpts = append(msgOpts, slackapi.MsgOptionDisableLinkUnfurl())
	}
	if !opts.UnfurlMedia {
		msgOpts = append(msgOpts, slackapi.MsgOptionDisableMediaUnfurl())
	}

	respChannel, respTimestamp, err := c.sdk.PostMessageContext(ctx, channel, msgOpts...)
	if err != nil {
		return nil, fmt.Errorf("post message: %w", err)
	}

	return &PostMessageResult{
		OK:        true,
		Channel:   respChannel,
		Timestamp: respTimestamp,
		Text:      opts.Text,
	}, nil
}

// EditMessage updates an existing message.
func (c *APIClient) EditMessage(ctx context.Context, channel, timestamp, text string) (*EditMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if timestamp == "" {
		return nil, ErrTimestampRequired
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	respChannel, respTimestamp, respText, err := c.sdk.UpdateMessageContext(
		ctx,
		channel,
		timestamp,
		slackapi.MsgOptionText(text, false),
	)
	if err != nil {
		return nil, fmt.Errorf("edit message: %w", err)
	}

	return &EditMessageResult{
		OK:        true,
		Channel:   respChannel,
		Timestamp: respTimestamp,
		Text:      respText,
	}, nil
}

// DeleteMessage deletes a message.
func (c *APIClient) DeleteMessage(ctx context.Context, channel, timestamp string) (*DeleteMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if timestamp == "" {
		return nil, ErrTimestampRequired
	}

	_, _, err := c.sdk.DeleteMessageContext(ctx, channel, timestamp)
	if err != nil {
		return nil, fmt.Errorf("delete message: %w", err)
	}

	return &DeleteMessageResult{
		OK:        true,
		Channel:   channel,
		Timestamp: timestamp,
	}, nil
}
