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
