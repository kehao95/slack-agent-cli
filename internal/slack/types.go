package slack

import (
	"fmt"

	slackapi "github.com/slack-go/slack"
)

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

// ReactionListResult represents the result of listing reactions on a message.
type ReactionListResult struct {
	OK        bool           `json:"ok"`
	Channel   string         `json:"channel"`
	ChannelID string         `json:"channel_id"`
	Timestamp string         `json:"ts"`
	Reactions []ReactionItem `json:"reactions"`
}

// ReactionItem represents a single reaction (emoji) with count and users.
type ReactionItem struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *ReactionListResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("Reactions on message in %s", r.Channel),
		fmt.Sprintf("Timestamp: %s", r.Timestamp),
		"───────────────────────────────",
	}

	if len(r.Reactions) == 0 {
		lines = append(lines, "No reactions on this message.")
		return lines
	}

	for _, reaction := range r.Reactions {
		userList := fmt.Sprintf("%d user(s)", reaction.Count)
		if len(reaction.Users) > 0 && len(reaction.Users) <= 5 {
			// Show user IDs if there are 5 or fewer
			userList = fmt.Sprintf("by: %v", reaction.Users)
		}
		lines = append(lines, fmt.Sprintf(":%s: × %d %s", reaction.Name, reaction.Count, userList))
	}

	return lines
}

// EmojiListResult represents the result of listing custom emoji.
type EmojiListResult struct {
	OK    bool              `json:"ok"`
	Emoji map[string]string `json:"emoji"`
	Count int               `json:"count"`
}

// EmojiItem represents a single emoji for easier display.
type EmojiItem struct {
	Name  string `json:"name"`
	Value string `json:"value"` // URL for custom emoji, alias for standard
}

// Lines implements the output.Printable interface for human-readable output.
func (r *EmojiListResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("Custom Emoji (%d)", r.Count),
		"───────────────────────────────",
	}

	if r.Count == 0 {
		lines = append(lines, "No custom emoji found.")
		return lines
	}

	// Sort emoji names for consistent output
	names := make([]string, 0, len(r.Emoji))
	for name := range r.Emoji {
		names = append(names, name)
	}

	// Display up to 50 emoji in human-readable mode
	displayCount := len(names)
	if displayCount > 50 {
		displayCount = 50
	}

	for i := 0; i < displayCount; i++ {
		name := names[i]
		value := r.Emoji[name]
		// Truncate long URLs for readability
		if len(value) > 60 {
			value = value[:57] + "..."
		}
		lines = append(lines, fmt.Sprintf(":%s: → %s", name, value))
	}

	if len(names) > 50 {
		lines = append(lines, fmt.Sprintf("\n... and %d more (default output is JSON with all items)", len(names)-50))
	}

	return lines
}

// ChannelJoinResult represents the result of joining a channel.
type ChannelJoinResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *ChannelJoinResult) Lines() []string {
	return []string{
		fmt.Sprintf("✓ Joined channel %s", r.Channel),
	}
}

// ChannelLeaveResult represents the result of leaving a channel.
type ChannelLeaveResult struct {
	OK        bool   `json:"ok"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *ChannelLeaveResult) Lines() []string {
	return []string{
		fmt.Sprintf("✓ Left channel %s", r.Channel),
	}
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

// ListChannelsParams controls ListChannels behavior.
type ListChannelsParams struct {
	Limit           int
	Cursor          string
	IncludeArchived bool
	Types           []string
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
