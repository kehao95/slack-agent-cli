package slack

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

type ConversationMutationResult struct {
	OK      bool              `json:"ok"`
	Action  string            `json:"action"`
	Channel *slackapi.Channel `json:"channel,omitempty"`
	ID      string            `json:"channel_id,omitempty"`
}

func (r *ConversationMutationResult) Lines() []string {
	id := r.ID
	name := ""
	if r.Channel != nil {
		id = r.Channel.ID
		name = r.Channel.Name
	}
	line := fmt.Sprintf("Conversation %s: %s", r.Action, id)
	if name != "" {
		line = fmt.Sprintf("Conversation %s: #%s (%s)", r.Action, name, id)
	}
	return []string{line}
}

type ConversationMembersResult struct {
	ChannelID  string   `json:"channel_id"`
	Members    []string `json:"members"`
	NextCursor string   `json:"next_cursor"`
}

func (r *ConversationMembersResult) Lines() []string {
	lines := []string{fmt.Sprintf("Members in %s (%d)", r.ChannelID, len(r.Members))}
	lines = append(lines, r.Members...)
	if r.NextCursor != "" {
		lines = append(lines, "Next cursor: "+r.NextCursor)
	}
	return lines
}

type OpenConversationResult struct {
	OK          bool              `json:"ok"`
	Channel     *slackapi.Channel `json:"channel"`
	NoOp        bool              `json:"no_op"`
	AlreadyOpen bool              `json:"already_open"`
}

func (r *OpenConversationResult) Lines() []string {
	id := ""
	if r.Channel != nil {
		id = r.Channel.ID
	}
	return []string{"Conversation open: " + id}
}

type CloseConversationResult struct {
	OK            bool   `json:"ok"`
	ChannelID     string `json:"channel_id"`
	NoOp          bool   `json:"no_op"`
	AlreadyClosed bool   `json:"already_closed"`
}

func (r *CloseConversationResult) Lines() []string {
	return []string{"Conversation closed: " + r.ChannelID}
}

type ConversationInfoResult struct {
	OK      bool              `json:"ok"`
	Channel *slackapi.Channel `json:"channel"`
}

func (r *ConversationInfoResult) Lines() []string {
	if r.Channel == nil {
		return []string{"Conversation not found"}
	}
	name := r.Channel.Name
	if name == "" {
		name = r.Channel.ID
	}
	return []string{fmt.Sprintf("Conversation: %s (%s)", name, r.Channel.ID)}
}

func (c *APIClient) ConversationInfo(ctx context.Context, channelID string, includeLocale, includeNumMembers bool) (*ConversationInfoResult, error) {
	channel, err := c.sdk.GetConversationInfoContext(ctx, &slackapi.GetConversationInfoInput{
		ChannelID: channelID, IncludeLocale: includeLocale, IncludeNumMembers: includeNumMembers,
	})
	if err != nil {
		return nil, fmt.Errorf("get conversation info: %w", err)
	}
	return &ConversationInfoResult{OK: true, Channel: channel}, nil
}

func (c *APIClient) CreateConversation(ctx context.Context, name string, private bool) (*ConversationMutationResult, error) {
	name = strings.TrimSpace(strings.TrimPrefix(name, "#"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	channel, err := c.sdk.CreateConversationContext(ctx, slackapi.CreateConversationParams{ChannelName: name, IsPrivate: private})
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "created", Channel: channel}, nil
}

func (c *APIClient) ArchiveConversation(ctx context.Context, channelID string) (*ConversationMutationResult, error) {
	if err := c.sdk.ArchiveConversationContext(ctx, channelID); err != nil {
		return nil, fmt.Errorf("archive conversation: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "archived", ID: channelID}, nil
}

func (c *APIClient) UnarchiveConversation(ctx context.Context, channelID string) (*ConversationMutationResult, error) {
	if err := c.sdk.UnArchiveConversationContext(ctx, channelID); err != nil {
		return nil, fmt.Errorf("unarchive conversation: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "unarchived", ID: channelID}, nil
}

func (c *APIClient) RenameConversation(ctx context.Context, channelID, name string) (*ConversationMutationResult, error) {
	name = strings.TrimSpace(strings.TrimPrefix(name, "#"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	channel, err := c.sdk.RenameConversationContext(ctx, channelID, name)
	if err != nil {
		return nil, fmt.Errorf("rename conversation: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "renamed", Channel: channel}, nil
}

func (c *APIClient) SetConversationTopic(ctx context.Context, channelID, topic string) (*ConversationMutationResult, error) {
	channel, err := c.sdk.SetTopicOfConversationContext(ctx, channelID, topic)
	if err != nil {
		return nil, fmt.Errorf("set conversation topic: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "topic_updated", Channel: channel}, nil
}

func (c *APIClient) SetConversationPurpose(ctx context.Context, channelID, purpose string) (*ConversationMutationResult, error) {
	channel, err := c.sdk.SetPurposeOfConversationContext(ctx, channelID, purpose)
	if err != nil {
		return nil, fmt.Errorf("set conversation purpose: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "purpose_updated", Channel: channel}, nil
}

func (c *APIClient) ListConversationMembers(ctx context.Context, channelID, cursor string, limit int) (*ConversationMembersResult, error) {
	members, nextCursor, err := c.sdk.GetUsersInConversationContext(ctx, &slackapi.GetUsersInConversationParameters{
		ChannelID: channelID, Cursor: cursor, Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list conversation members: %w", err)
	}
	return &ConversationMembersResult{ChannelID: channelID, Members: members, NextCursor: nextCursor}, nil
}

func (c *APIClient) InviteConversationUsers(ctx context.Context, channelID string, users []string) (*ConversationMutationResult, error) {
	if len(users) == 0 {
		return nil, fmt.Errorf("at least one user is required")
	}
	channel, err := c.sdk.InviteUsersToConversationContext(ctx, channelID, users...)
	if err != nil {
		return nil, fmt.Errorf("invite conversation users: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "users_invited", Channel: channel}, nil
}

func (c *APIClient) KickConversationUser(ctx context.Context, channelID, userID string) (*ConversationMutationResult, error) {
	if err := c.sdk.KickUserFromConversationContext(ctx, channelID, userID); err != nil {
		return nil, fmt.Errorf("kick conversation user: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "user_removed", ID: channelID}, nil
}

func (c *APIClient) OpenConversation(ctx context.Context, channelID string, users []string) (*OpenConversationResult, error) {
	if channelID == "" && len(users) == 0 {
		return nil, fmt.Errorf("channel or users is required")
	}
	channel, noOp, alreadyOpen, err := c.sdk.OpenConversationContext(ctx, &slackapi.OpenConversationParameters{
		ChannelID: channelID, Users: users, ReturnIM: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open conversation: %w", err)
	}
	if channel == nil || strings.TrimSpace(channel.ID) == "" {
		return nil, fmt.Errorf("open conversation: Slack response did not include a conversation ID")
	}
	return &OpenConversationResult{OK: true, Channel: channel, NoOp: noOp, AlreadyOpen: alreadyOpen}, nil
}

func (c *APIClient) CloseConversation(ctx context.Context, channelID string) (*CloseConversationResult, error) {
	noOp, alreadyClosed, err := c.sdk.CloseConversationContext(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("close conversation: %w", err)
	}
	return &CloseConversationResult{OK: true, ChannelID: channelID, NoOp: noOp, AlreadyClosed: alreadyClosed}, nil
}

func (c *APIClient) MarkConversationRead(ctx context.Context, channelID, timestamp string) (*ConversationMutationResult, error) {
	if strings.TrimSpace(timestamp) == "" {
		return nil, fmt.Errorf("timestamp is required")
	}
	if err := c.sdk.MarkConversationContext(ctx, channelID, timestamp); err != nil {
		return nil, fmt.Errorf("mark conversation read: %w", err)
	}
	return &ConversationMutationResult{OK: true, Action: "marked", ID: channelID}, nil
}
