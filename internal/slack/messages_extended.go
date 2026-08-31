package slack

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

type PermalinkResult struct {
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"ts"`
	Permalink string `json:"permalink"`
}

func (r *PermalinkResult) Lines() []string { return []string{r.Permalink} }

type EphemeralMessageResult struct {
	OK        bool   `json:"ok"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Timestamp string `json:"message_ts"`
}

func (r *EphemeralMessageResult) Lines() []string {
	return []string{fmt.Sprintf("Ephemeral message sent to %s in %s (%s)", r.UserID, r.ChannelID, r.Timestamp)}
}

type ScheduledMessageResult struct {
	OK                 bool   `json:"ok"`
	ChannelID          string `json:"channel_id"`
	ScheduledMessageID string `json:"scheduled_message_id"`
	PostAt             string `json:"post_at,omitempty"`
}

func (r *ScheduledMessageResult) Lines() []string {
	return []string{fmt.Sprintf("Scheduled message %s in %s", r.ScheduledMessageID, r.ChannelID)}
}

type ScheduledMessagesResult struct {
	Messages   []slackapi.ScheduledMessage `json:"scheduled_messages"`
	NextCursor string                      `json:"next_cursor"`
}

func (r *ScheduledMessagesResult) Lines() []string {
	lines := []string{fmt.Sprintf("Scheduled messages (%d)", len(r.Messages))}
	for _, message := range r.Messages {
		lines = append(lines, fmt.Sprintf("%s %s %d %s", message.ID, message.Channel, message.PostAt, message.Text))
	}
	if r.NextCursor != "" {
		lines = append(lines, "Next cursor: "+r.NextCursor)
	}
	return lines
}

type DeleteScheduledMessageResult struct {
	OK                 bool   `json:"ok"`
	ChannelID          string `json:"channel_id"`
	ScheduledMessageID string `json:"scheduled_message_id"`
}

func (r *DeleteScheduledMessageResult) Lines() []string {
	return []string{"Deleted scheduled message " + r.ScheduledMessageID}
}

type StreamMessageResult struct {
	OK        bool   `json:"ok"`
	Action    string `json:"action"`
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"ts"`
}

func (r *StreamMessageResult) Lines() []string {
	return []string{fmt.Sprintf("Stream %s in %s (%s)", r.Action, r.ChannelID, r.Timestamp)}
}

type StreamStartOptions struct {
	ThreadTS        string
	RecipientTeamID string
	RecipientUserID string
}

type StreamUpdateOptions struct {
	Markdown string
	Blocks   []slackapi.Block
}

func (c *APIClient) GetMessagePermalink(ctx context.Context, channelID, timestamp string) (*PermalinkResult, error) {
	if channelID == "" || timestamp == "" {
		return nil, fmt.Errorf("channel and timestamp are required")
	}
	permalink, err := c.sdk.GetPermalinkContext(ctx, &slackapi.PermalinkParameters{Channel: channelID, Ts: timestamp})
	if err != nil {
		return nil, fmt.Errorf("get message permalink: %w", err)
	}
	return &PermalinkResult{ChannelID: channelID, Timestamp: timestamp, Permalink: permalink}, nil
}

func (c *APIClient) PostEphemeralMessage(ctx context.Context, channelID, userID string, opts PostMessageOptions) (*EphemeralMessageResult, error) {
	if channelID == "" || userID == "" {
		return nil, fmt.Errorf("channel and user are required")
	}
	if opts.Text == "" && len(opts.Blocks) == 0 {
		return nil, ErrTextRequired
	}
	timestamp, err := c.sdk.PostEphemeralContext(ctx, channelID, userID, standardMessageOptions(opts)...)
	if err != nil {
		return nil, fmt.Errorf("post ephemeral message: %w", err)
	}
	return &EphemeralMessageResult{OK: true, ChannelID: channelID, UserID: userID, Timestamp: timestamp}, nil
}

func (c *APIClient) ScheduleMessage(ctx context.Context, channelID, postAt string, opts PostMessageOptions) (*ScheduledMessageResult, error) {
	if channelID == "" {
		return nil, ErrChannelRequired
	}
	if strings.TrimSpace(postAt) == "" {
		return nil, fmt.Errorf("post-at is required")
	}
	if opts.Text == "" && len(opts.Blocks) == 0 {
		return nil, ErrTextRequired
	}
	responseChannel, scheduledID, err := c.sdk.ScheduleMessageContext(ctx, channelID, postAt, standardMessageOptions(opts)...)
	if err != nil {
		return nil, fmt.Errorf("schedule message: %w", err)
	}
	return &ScheduledMessageResult{OK: true, ChannelID: responseChannel, ScheduledMessageID: scheduledID, PostAt: postAt}, nil
}

func (c *APIClient) ListScheduledMessages(ctx context.Context, channelID, cursor, oldest, latest string, limit int) (*ScheduledMessagesResult, error) {
	messages, nextCursor, err := c.sdk.GetScheduledMessagesContext(ctx, &slackapi.GetScheduledMessagesParameters{
		Channel: channelID, Cursor: cursor, Oldest: oldest, Latest: latest, Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled messages: %w", err)
	}
	return &ScheduledMessagesResult{Messages: messages, NextCursor: nextCursor}, nil
}

func (c *APIClient) DeleteScheduledMessage(ctx context.Context, channelID, scheduledID string, asUser bool) (*DeleteScheduledMessageResult, error) {
	if channelID == "" || scheduledID == "" {
		return nil, fmt.Errorf("channel and scheduled message ID are required")
	}
	ok, err := c.sdk.DeleteScheduledMessageContext(ctx, &slackapi.DeleteScheduledMessageParameters{
		Channel: channelID, ScheduledMessageID: scheduledID, AsUser: asUser,
	})
	if err != nil {
		return nil, fmt.Errorf("delete scheduled message: %w", err)
	}
	return &DeleteScheduledMessageResult{OK: ok, ChannelID: channelID, ScheduledMessageID: scheduledID}, nil
}

func (c *APIClient) StartMessageStream(ctx context.Context, channelID string, opts StreamStartOptions) (*StreamMessageResult, error) {
	options := make([]slackapi.MsgOption, 0, 3)
	if opts.ThreadTS != "" {
		options = append(options, slackapi.MsgOptionTS(opts.ThreadTS))
	}
	if opts.RecipientTeamID != "" {
		options = append(options, slackapi.MsgOptionRecipientTeamID(opts.RecipientTeamID))
	}
	if opts.RecipientUserID != "" {
		options = append(options, slackapi.MsgOptionRecipientUserID(opts.RecipientUserID))
	}
	responseChannel, timestamp, err := c.sdk.StartStreamContext(ctx, channelID, options...)
	if err != nil {
		return nil, fmt.Errorf("start message stream: %w", err)
	}
	return &StreamMessageResult{OK: true, Action: "started", ChannelID: responseChannel, Timestamp: timestamp}, nil
}

func (c *APIClient) AppendMessageStream(ctx context.Context, channelID, timestamp, markdown string) (*StreamMessageResult, error) {
	if strings.TrimSpace(markdown) == "" {
		return nil, fmt.Errorf("markdown is required")
	}
	responseChannel, responseTimestamp, err := c.sdk.AppendStreamContext(ctx, channelID, timestamp, slackapi.MsgOptionMarkdownText(markdown))
	if err != nil {
		return nil, fmt.Errorf("append message stream: %w", err)
	}
	return &StreamMessageResult{OK: true, Action: "appended", ChannelID: responseChannel, Timestamp: responseTimestamp}, nil
}

func (c *APIClient) StopMessageStream(ctx context.Context, channelID, timestamp string, opts StreamUpdateOptions) (*StreamMessageResult, error) {
	options := make([]slackapi.MsgOption, 0, 2)
	if opts.Markdown != "" {
		options = append(options, slackapi.MsgOptionMarkdownText(opts.Markdown))
	}
	if len(opts.Blocks) > 0 {
		options = append(options, slackapi.MsgOptionBlocks(opts.Blocks...))
	}
	responseChannel, responseTimestamp, err := c.sdk.StopStreamContext(ctx, channelID, timestamp, options...)
	if err != nil {
		return nil, fmt.Errorf("stop message stream: %w", err)
	}
	return &StreamMessageResult{OK: true, Action: "stopped", ChannelID: responseChannel, Timestamp: responseTimestamp}, nil
}

func standardMessageOptions(opts PostMessageOptions) []slackapi.MsgOption {
	options := []slackapi.MsgOption{slackapi.MsgOptionText(opts.Text, false)}
	if opts.ThreadTS != "" {
		options = append(options, slackapi.MsgOptionTS(opts.ThreadTS))
	}
	if len(opts.Blocks) > 0 {
		options = append(options, slackapi.MsgOptionBlocks(opts.Blocks...))
	}
	if opts.AsUser {
		options = append(options, slackapi.MsgOptionAsUser(true))
	}
	if !opts.UnfurlLinks {
		options = append(options, slackapi.MsgOptionDisableLinkUnfurl())
	}
	if !opts.UnfurlMedia {
		options = append(options, slackapi.MsgOptionDisableMediaUnfurl())
	}
	return options
}
