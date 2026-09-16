package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
)

// MessageFetcher handles message retrieval logic.
type MessageFetcher struct {
	client Client
}

// GetMessageResult is the stable envelope returned by messages get.
type GetMessageResult struct {
	OK        bool               `json:"ok"`
	Channel   string             `json:"channel"`
	ChannelID string             `json:"channel_id,omitempty"`
	TS        string             `json:"ts"`
	Message   slackapi.Message   `json:"message"`
	Thread    []slackapi.Message `json:"thread,omitempty"`
}

func (r *GetMessageResult) Lines() []string {
	lines := []string{fmt.Sprintf("Message %s in %s", r.TS, r.Channel), r.Message.Text}
	if len(r.Thread) > 0 {
		lines = append(lines, fmt.Sprintf("Thread messages: %d", len(r.Thread)))
	}
	return lines
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
	options.IncludeAllMetadata = params.IncludeAllMetadata

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
	opts.IncludeAllMetadata = params.IncludeAllMetadata
	msgs, hasMore, nextCursor, err := c.sdk.GetConversationRepliesContext(ctx, opts)
	return msgs, hasMore, nextCursor, err
}

// GetMessage fetches one message by exact channel/timestamp and optionally its full thread.
func (c *APIClient) GetMessage(ctx context.Context, channel, timestamp string, includeThread bool, threadRoot ...string) (*GetMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if timestamp == "" {
		return nil, ErrTimestampRequired
	}
	root := ""
	if len(threadRoot) > 0 {
		root = strings.TrimSpace(threadRoot[0])
	}
	if root != "" {
		thread, err := c.getAllThreadReplies(ctx, channel, root)
		if err != nil {
			return nil, fmt.Errorf("get message thread: %w", err)
		}
		for _, message := range thread {
			if message.Timestamp == timestamp {
				result := &GetMessageResult{OK: true, Channel: channel, ChannelID: channel, TS: timestamp, Message: message}
				if includeThread {
					result.Thread = thread
				}
				return result, nil
			}
		}
		return nil, fmt.Errorf("message %s was not found in thread %s", timestamp, root)
	}
	response, err := c.ListConversationsHistory(ctx, HistoryParams{Channel: channel, Limit: 1, Oldest: timestamp, Latest: timestamp, Inclusive: true})
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if response == nil || len(response.Messages) == 0 {
		return nil, fmt.Errorf("message %s was not found in %s; for a thread reply, provide --thread-ts", timestamp, channel)
	}
	if response.Messages[0].Timestamp != timestamp {
		return nil, fmt.Errorf("message %s was not found in %s; for a thread reply, provide --thread-ts", timestamp, channel)
	}
	result := &GetMessageResult{OK: true, Channel: channel, ChannelID: channel, TS: timestamp, Message: response.Messages[0]}
	if includeThread {
		thread, err := c.getAllThreadReplies(ctx, channel, timestamp)
		if err != nil {
			return nil, fmt.Errorf("get message thread: %w", err)
		}
		result.Thread = thread
	}
	return result, nil
}

func (c *APIClient) getAllThreadReplies(ctx context.Context, channel, root string) ([]slackapi.Message, error) {
	var all []slackapi.Message
	cursor := ""
	seen := map[string]bool{}
	for {
		page, more, next, err := c.ListThreadReplies(ctx, ThreadParams{Channel: channel, Thread: root, Limit: 100, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if !more || next == "" {
			return all, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("Slack returned repeated message cursor %q", next)
		}
		seen[next] = true
		cursor = next
	}
}

// PostMessage sends a message to a channel.
func (c *APIClient) PostMessage(ctx context.Context, channel string, opts PostMessageOptions) (*PostMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if opts.Text == "" && len(opts.Blocks) == 0 && len(opts.Attachments) == 0 && opts.Metadata == nil {
		return nil, ErrTextRequired
	}

	if opts.ClientMsgID != "" {
		payload := map[string]interface{}{"channel": channel, "text": opts.Text, "client_msg_id": opts.ClientMsgID}
		if opts.ThreadTS != "" {
			payload["thread_ts"] = opts.ThreadTS
		}
		if len(opts.Blocks) > 0 {
			payload["blocks"] = opts.Blocks
		}
		if len(opts.Attachments) > 0 {
			payload["attachments"] = opts.Attachments
		}
		if opts.Metadata != nil {
			payload["metadata"] = opts.Metadata
		}
		if opts.ReplyBroadcast {
			payload["reply_broadcast"] = true
		}
		payload["unfurl_links"] = opts.UnfurlLinks
		payload["unfurl_media"] = opts.UnfurlMedia
		if opts.AsUser {
			payload["as_user"] = true
		}
		raw, err := c.CallAPI(ctx, "chat.postMessage", payload, CallAPIOptions{MaxRetries: 3})
		if err != nil {
			return nil, fmt.Errorf("post message: %w", err)
		}
		var response struct {
			OK        bool   `json:"ok"`
			Channel   string `json:"channel"`
			Timestamp string `json:"ts"`
			Text      string `json:"text"`
			Message   struct {
				Text string `json:"text"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode posted message: %w", err)
		}
		if response.Text == "" {
			response.Text = response.Message.Text
		}
		return &PostMessageResult{OK: response.OK, Channel: response.Channel, Timestamp: response.Timestamp, Text: response.Text}, nil
	}

	msgOpts := standardMessageOptions(opts)

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
	return c.EditMessageWithOptions(ctx, channel, timestamp, PostMessageOptions{Text: text})
}

// EditMessageWithOptions updates text, blocks, attachments, and/or metadata.
func (c *APIClient) EditMessageWithOptions(ctx context.Context, channel, timestamp string, opts PostMessageOptions) (*EditMessageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if timestamp == "" {
		return nil, ErrTimestampRequired
	}
	if !opts.MetadataClear && !opts.TextSet && !opts.BlocksSet && !opts.AttachmentsSet && opts.Text == "" && len(opts.Blocks) == 0 && len(opts.Attachments) == 0 && opts.Metadata == nil {
		return nil, fmt.Errorf("at least one of text, blocks, attachments, or metadata is required")
	}
	if opts.MetadataClear {
		payload := map[string]interface{}{"channel": channel, "ts": timestamp, "metadata": map[string]interface{}{}}
		if opts.Text != "" || opts.TextSet {
			payload["text"] = opts.Text
		}
		if opts.BlocksSet || len(opts.Blocks) > 0 {
			if opts.Blocks == nil {
				opts.Blocks = []slackapi.Block{}
			}
			payload["blocks"] = opts.Blocks
		}
		if opts.AttachmentsSet || len(opts.Attachments) > 0 {
			if opts.Attachments == nil {
				opts.Attachments = []slackapi.Attachment{}
			}
			payload["attachments"] = opts.Attachments
		}
		raw, err := c.CallAPI(ctx, "chat.update", payload, CallAPIOptions{MaxRetries: 3})
		if err != nil {
			return nil, fmt.Errorf("edit message: %w", err)
		}
		var response struct {
			Channel   string `json:"channel"`
			Timestamp string `json:"ts"`
			Text      string `json:"text"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode edited message: %w", err)
		}
		return &EditMessageResult{OK: true, Channel: response.Channel, Timestamp: response.Timestamp, Text: response.Text}, nil
	}

	messageOptions := make([]slackapi.MsgOption, 0, 4)
	if opts.Text != "" || opts.TextSet {
		messageOptions = append(messageOptions, slackapi.MsgOptionText(opts.Text, false))
	}
	if len(opts.Blocks) > 0 || opts.BlocksSet {
		messageOptions = append(messageOptions, slackapi.MsgOptionBlocks(opts.Blocks...))
	}
	if len(opts.Attachments) > 0 {
		messageOptions = append(messageOptions, slackapi.MsgOptionAttachments(opts.Attachments...))
	} else if opts.AttachmentsSet {
		messageOptions = append(messageOptions, slackapi.MsgOptionAttachments([]slackapi.Attachment{}...))
	}
	if opts.Metadata != nil {
		messageOptions = append(messageOptions, slackapi.MsgOptionMetadata(*opts.Metadata))
	}
	respChannel, respTimestamp, respText, err := c.sdk.UpdateMessageContext(ctx, channel, timestamp, messageOptions...)
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
