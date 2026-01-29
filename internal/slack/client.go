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

// MessageClient provides message posting capabilities.
type MessageClient interface {
	PostMessage(ctx context.Context, channel string, opts PostMessageOptions) (*PostMessageResult, error)
	EditMessage(ctx context.Context, channel, timestamp, text string) (*EditMessageResult, error)
	DeleteMessage(ctx context.Context, channel, timestamp string) (*DeleteMessageResult, error)
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

// ReactionClient provides reaction capabilities.
type ReactionClient interface {
	AddReaction(ctx context.Context, channel, timestamp, emoji string) error
	RemoveReaction(ctx context.Context, channel, timestamp, emoji string) error
}

// PinClient provides pin capabilities.
type PinClient interface {
	AddPin(ctx context.Context, channel, timestamp string) error
	RemovePin(ctx context.Context, channel, timestamp string) error
	ListPins(ctx context.Context, channel string) (*PinListResult, error)
}

// FullClient combines all client capabilities.
type FullClient interface {
	ChannelClient
	UserClient
}

// PostMessageOptions wraps arguments for posting a message.
type PostMessageOptions struct {
	Text        string
	ThreadTS    string
	Blocks      []slackapi.Block
	UnfurlLinks bool
	UnfurlMedia bool
}

// PostMessageResult represents the result of posting a message.
type PostMessageResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
	Text      string `json:"text,omitempty"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *PostMessageResult) Lines() []string {
	lines := []string{
		"Message sent successfully",
		fmt.Sprintf("Channel: %s", r.Channel),
		fmt.Sprintf("Timestamp: %s", r.Timestamp),
	}
	return lines
}

// EditMessageResult represents the result of editing a message.
type EditMessageResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
	Text      string `json:"text"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *EditMessageResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("✓ Message updated in %s", r.Channel),
		fmt.Sprintf("Timestamp: %s", r.Timestamp),
	}
	return lines
}

// DeleteMessageResult represents the result of deleting a message.
type DeleteMessageResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *DeleteMessageResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("✓ Message deleted from %s", r.Channel),
		fmt.Sprintf("Timestamp: %s", r.Timestamp),
	}
	return lines
}

// ReactionResult represents the result of adding or removing a reaction.
type ReactionResult struct {
	OK        bool   `json:"ok"`
	Action    string `json:"action"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"ts"`
	Emoji     string `json:"emoji"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *ReactionResult) Lines() []string {
	var actionText string
	if r.Action == "add" {
		actionText = fmt.Sprintf("✓ Added :%s: to message in %s", r.Emoji, r.Channel)
	} else {
		actionText = fmt.Sprintf("✓ Removed :%s: from message in %s", r.Emoji, r.Channel)
	}
	return []string{actionText}
}

// PinResult represents the result of adding or removing a pin.
type PinResult struct {
	OK        bool   `json:"ok"`
	Action    string `json:"action"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Timestamp string `json:"ts"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *PinResult) Lines() []string {
	var actionText string
	if r.Action == "add" {
		actionText = fmt.Sprintf("✓ Pinned message in %s", r.Channel)
	} else {
		actionText = fmt.Sprintf("✓ Unpinned message from %s", r.Channel)
	}
	return []string{
		actionText,
		fmt.Sprintf("Timestamp: %s", r.Timestamp),
	}
}

// PinListResult represents the result of listing pins.
type PinListResult struct {
	OK      bool         `json:"ok"`
	Channel string       `json:"channel"`
	Items   []PinnedItem `json:"items"`
}

// PinnedItem represents a pinned item in a channel.
type PinnedItem struct {
	Type      string   `json:"type"`
	Channel   string   `json:"channel,omitempty"`
	Message   *Message `json:"message,omitempty"`
	CreatedBy string   `json:"created_by"`
	Created   int64    `json:"created"`
}

// Message represents a simplified Slack message for pin display.
type Message struct {
	Timestamp string `json:"ts"`
	Text      string `json:"text"`
	User      string `json:"user"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *PinListResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("Pinned Messages in %s (%d)", r.Channel, len(r.Items)),
		"───────────────────────────────",
	}

	if len(r.Items) == 0 {
		lines = append(lines, "No pinned messages.")
		return lines
	}

	for _, item := range r.Items {
		if item.Type == "message" && item.Message != nil {
			msg := item.Message
			// Format: [timestamp] @user: text
			userDisplay := msg.User
			if userDisplay == "" {
				userDisplay = "unknown"
			}
			text := msg.Text
			if len(text) > 100 {
				text = text[:97] + "..."
			}
			lines = append(lines, fmt.Sprintf("[%s] @%s: %s", msg.Timestamp, userDisplay, text))
		} else {
			// Non-message pins (files, etc.)
			lines = append(lines, fmt.Sprintf("[%s] %s item", item.Type, item.Type))
		}
	}

	return lines
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

// New creates a new APIClient using the provided user token.
func New(userToken string) *APIClient {
	return &APIClient{sdk: slackapi.New(userToken)}
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

// PostMessage sends a message to a channel.
func (c *APIClient) PostMessage(ctx context.Context, channel string, opts PostMessageOptions) (*PostMessageResult, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if opts.Text == "" && len(opts.Blocks) == 0 {
		return nil, fmt.Errorf("either text or blocks is required")
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
		return nil, fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return nil, fmt.Errorf("timestamp is required")
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
		return nil, fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return nil, fmt.Errorf("timestamp is required")
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

// AddReaction adds an emoji reaction to a message.
func (c *APIClient) AddReaction(ctx context.Context, channel, timestamp, emoji string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.AddReactionContext(ctx, emoji, itemRef)
}

// RemoveReaction removes an emoji reaction from a message.
func (c *APIClient) RemoveReaction(ctx context.Context, channel, timestamp, emoji string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.RemoveReactionContext(ctx, emoji, itemRef)
}

// ListChannels fetches channels the calling user is a member of.
// Uses users.conversations API which works with channels:read scope on user tokens.
// Note: private_channel type requires groups:read scope, im type requires im:read scope.
func (c *APIClient) ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error) {
	convParams := &slackapi.GetConversationsForUserParameters{
		Limit:           params.Limit,
		Cursor:          params.Cursor,
		ExcludeArchived: !params.IncludeArchived,
	}
	// Only set types if explicitly provided - this avoids scope issues
	// When no types are specified, the API defaults to public channels only
	if len(params.Types) > 0 {
		convParams.Types = append(convParams.Types, params.Types...)
	}
	channels, nextCursor, err := c.sdk.GetConversationsForUserContext(ctx, convParams)
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

// GetUserGroups fetches all user groups from the workspace.
func (c *APIClient) GetUserGroups(ctx context.Context) ([]slackapi.UserGroup, error) {
	groups, err := c.sdk.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	return groups, nil
}

// ListChannelsPaginated provides a simpler interface for cache population.
// Returns public channels the user is a member of (uses users.conversations API).
// Note: Only fetches public_channel type to work with channels:read scope.
func (c *APIClient) ListChannelsPaginated(ctx context.Context, cursor string, limit int) ([]slackapi.Channel, string, int, error) {
	channels, nextCursor, err := c.ListChannels(ctx, ListChannelsParams{
		Limit:           limit,
		Cursor:          cursor,
		IncludeArchived: false,
		Types:           []string{"public_channel"},
	})
	if err != nil {
		return nil, "", 0, err
	}

	return channels, nextCursor, len(channels), nil
}

// AuthTestResponse contains the result of an auth.test API call.
type AuthTestResponse struct {
	OK     bool   `json:"ok"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	BotID  string `json:"bot_id,omitempty"`
	IsBot  bool   `json:"is_bot"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *AuthTestResponse) Lines() []string {
	lines := []string{
		"Authentication Test",
		"-------------------",
		fmt.Sprintf("Status: %s", statusString(r.OK)),
		fmt.Sprintf("Team: %s (%s)", r.Team, r.TeamID),
		fmt.Sprintf("User: %s (%s)", r.User, r.UserID),
		fmt.Sprintf("Workspace URL: %s", r.URL),
	}
	if r.BotID != "" {
		lines = append(lines, fmt.Sprintf("Bot ID: %s", r.BotID))
	}
	return lines
}

func statusString(ok bool) string {
	if ok {
		return "✓ Valid"
	}
	return "✗ Invalid"
}

// AuthTest verifies the user token is valid.
func (c *APIClient) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	resp, err := c.sdk.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}
	return &AuthTestResponse{
		OK:     true,
		URL:    resp.URL,
		Team:   resp.Team,
		User:   resp.User,
		TeamID: resp.TeamID,
		UserID: resp.UserID,
		BotID:  resp.BotID,
	}, nil
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

// SearchClient provides message search capabilities (requires user token).
type SearchClient interface {
	SearchMessages(ctx context.Context, query string, params SearchParams) (*SearchResult, error)
}

// SearchParams wraps arguments for search.messages.
type SearchParams struct {
	Count     int
	Page      int
	SortBy    string // "score" or "timestamp"
	SortDir   string // "asc" or "desc"
	Highlight bool
}

// SearchResult represents the search.messages API response.
type SearchResult struct {
	Query    string         `json:"query"`
	Messages SearchMessages `json:"messages"`
}

// SearchMessages contains the list of matching messages.
type SearchMessages struct {
	Total   int           `json:"total"`
	Matches []SearchMatch `json:"matches"`
}

// SearchMatch represents a single search result.
type SearchMatch struct {
	Type      string        `json:"type"`
	Channel   SearchChannel `json:"channel"`
	User      string        `json:"user"`
	Username  string        `json:"username"`
	Timestamp string        `json:"ts"`
	Text      string        `json:"text"`
	Permalink string        `json:"permalink"`
}

// SearchChannel contains channel metadata for a search result.
type SearchChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserAPIClient wraps operations requiring user token.
type UserAPIClient struct {
	sdk *slackapi.Client
}

// NewUserClient creates a new UserAPIClient using the provided user token.
func NewUserClient(userToken string) *UserAPIClient {
	return &UserAPIClient{sdk: slackapi.New(userToken)}
}

// SearchMessages searches messages across the workspace using search.messages API.
func (c *UserAPIClient) SearchMessages(ctx context.Context, query string, params SearchParams) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	searchParams := slackapi.SearchParameters{
		Sort:          params.SortBy,
		SortDirection: params.SortDir,
		Count:         params.Count,
		Page:          params.Page,
		Highlight:     params.Highlight,
	}

	messages, err := c.sdk.SearchMessagesContext(ctx, query, searchParams)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}

	// Map slack-go response to our internal structure
	result := &SearchResult{
		Query: query,
		Messages: SearchMessages{
			Total:   messages.Total,
			Matches: make([]SearchMatch, len(messages.Matches)),
		},
	}

	for i, match := range messages.Matches {
		result.Messages.Matches[i] = SearchMatch{
			Type: match.Type,
			Channel: SearchChannel{
				ID:   match.Channel.ID,
				Name: match.Channel.Name,
			},
			User:      match.User,
			Username:  match.Username,
			Timestamp: match.Timestamp,
			Text:      match.Text,
			Permalink: match.Permalink,
		}
	}

	return result, nil
}

// Lines implements the output.Printable interface for human-readable search results.
func (r *SearchResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("Search Results for \"%s\" (%d matches)", r.Query, r.Messages.Total),
		"───────────────────────────────────────────────────",
	}

	if len(r.Messages.Matches) == 0 {
		lines = append(lines, "No messages found.")
		return lines
	}

	for _, match := range r.Messages.Matches {
		// Format timestamp
		ts := match.Timestamp
		if len(ts) > 10 {
			// Convert Slack timestamp (seconds.microseconds) to readable format
			// For simplicity, just show the timestamp as-is
			// In production, you'd parse this properly
		}

		channelName := match.Channel.Name
		if channelName == "" {
			channelName = match.Channel.ID
		}

		username := match.Username
		if username == "" {
			username = match.User
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[%s] #%s @%s:", ts, channelName, username))
		lines = append(lines, fmt.Sprintf("  %s", match.Text))
		if match.Permalink != "" {
			lines = append(lines, fmt.Sprintf("  %s", match.Permalink))
		}
	}

	return lines
}

// AddPin pins a message to a channel.
func (c *APIClient) AddPin(ctx context.Context, channel, timestamp string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.AddPinContext(ctx, channel, itemRef)
}

// RemovePin removes a pin from a message.
func (c *APIClient) RemovePin(ctx context.Context, channel, timestamp string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.RemovePinContext(ctx, channel, itemRef)
}

// ListPins lists all pinned items in a channel.
func (c *APIClient) ListPins(ctx context.Context, channel string) (*PinListResult, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}

	items, _, err := c.sdk.ListPinsContext(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}

	// Convert slack-go Items to our PinnedItem structure
	pinnedItems := make([]PinnedItem, 0, len(items))
	for _, item := range items {
		pinnedItem := PinnedItem{
			Type:    item.Type,
			Channel: item.Channel,
		}

		// If it's a message, convert the message data
		if item.Message != nil {
			pinnedItem.Message = &Message{
				Timestamp: item.Message.Timestamp,
				Text:      item.Message.Text,
				User:      item.Message.User,
			}
		}

		pinnedItems = append(pinnedItems, pinnedItem)
	}

	return &PinListResult{
		OK:      true,
		Channel: channel,
		Items:   pinnedItems,
	}, nil
}
