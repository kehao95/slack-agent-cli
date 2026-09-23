package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// PostMessageOptions wraps arguments for posting a message.
type PostMessageOptions struct {
	Text           string
	ThreadTS       string
	Blocks         []slackapi.Block
	Attachments    []slackapi.Attachment
	Metadata       *slackapi.SlackMetadata
	MetadataClear  bool
	ReplyBroadcast bool
	ClientMsgID    string
	TextSet        bool
	BlocksSet      bool
	AttachmentsSet bool
	UnfurlLinks    bool
	UnfurlMedia    bool
	AsUser         bool
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
			// This surface only has an ID, not an authoritative username.
			userDisplay := msg.User
			if userDisplay == "" {
				userDisplay = "unknown"
			}
			text := msg.Text
			if len(text) > 100 {
				text = text[:97] + "..."
			}
			lines = append(lines, fmt.Sprintf("[%s] %s: %s", msg.Timestamp, userDisplay, text))
		} else {
			// Non-message pins (files, etc.)
			lines = append(lines, fmt.Sprintf("[%s] %s item", item.Type, item.Type))
		}
	}

	return lines
}

// HistoryParams wraps the arguments to conversations.history.
type HistoryParams struct {
	Channel            string
	Cursor             string
	Limit              int
	Latest             string
	Oldest             string
	Inclusive          bool
	IncludeAllMetadata bool
}

// ThreadParams wraps arguments for conversations.replies.
type ThreadParams struct {
	Channel            string
	Cursor             string
	Limit              int
	Latest             string
	Oldest             string
	Thread             string
	IncludeAllMetadata bool
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
	OK       bool   `json:"ok"`
	URL      string `json:"url"`
	Team     string `json:"team"`
	User     string `json:"user"`
	TeamID   string `json:"team_id"`
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Role     string `json:"role,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
	IsBot    bool   `json:"is_bot"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *AuthTestResponse) Lines() []string {
	lines := []string{
		"Authentication Test",
		"-------------------",
		fmt.Sprintf("Status: %s", statusString(r.OK)),
		fmt.Sprintf("Team: %s (%s)", r.Team, r.TeamID),
		fmt.Sprintf("User ID: %s", r.UserID),
		fmt.Sprintf("Workspace URL: %s", r.URL),
	}
	if r.Username != "" {
		lines = append(lines, fmt.Sprintf("Username: %s", r.Username))
	}
	if r.Role != "" {
		lines = append(lines, fmt.Sprintf("Role: %s", r.Role))
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
	Query           string                `json:"query"`
	Messages        SearchMessages        `json:"messages"`
	Page            int                   `json:"page"`
	PageCount       int                   `json:"page_count"`
	NextPage        int                   `json:"next_page,omitempty"`
	HasMore         bool                  `json:"has_more"`
	userResolver    SearchUserResolver    `json:"-"`
	channelResolver SearchChannelResolver `json:"-"`
	ctx             context.Context       `json:"-"`
	rawJSON         bool                  `json:"-"`
}

// SearchMessages contains the list of matching messages.
type SearchMessages struct {
	Total   int           `json:"total"`
	Matches []SearchMatch `json:"matches"`
}

// SearchMatch represents a single native search result. Username may contain a
// message alias; normalized output replaces it only with a resolved account handle.
type SearchMatch struct {
	Type        string                `json:"type"`
	Channel     SearchChannel         `json:"channel"`
	User        string                `json:"user"`
	Username    string                `json:"username"`
	Timestamp   string                `json:"ts"`
	Text        string                `json:"text"`
	Permalink   string                `json:"permalink"`
	Attachments []slackapi.Attachment `json:"attachments,omitempty"`
	Blocks      slackapi.Blocks       `json:"blocks,omitempty"`
}

// SearchChannel contains channel metadata for a search result.
type SearchChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchUserResolver resolves search result user IDs to names.
type SearchUserResolver interface {
	GetDisplayName(ctx context.Context, userID string) string
	GetMentionName(ctx context.Context, userID string) string
}

// SearchChannelResolver resolves search result channel IDs to names.
type SearchChannelResolver interface {
	ResolveName(ctx context.Context, channelID string) string
}

// SetUserResolver configures JSON user enrichment for search results.
func (r *SearchResult) SetUserResolver(ctx context.Context, resolver SearchUserResolver) {
	r.ctx = ctx
	r.userResolver = resolver
}

// SetChannelResolver configures JSON channel enrichment for search results.
func (r *SearchResult) SetChannelResolver(ctx context.Context, resolver SearchChannelResolver) {
	r.ctx = ctx
	r.channelResolver = resolver
}

// SetRawJSON disables enrichment and retains native search identity fields.
func (r *SearchResult) SetRawJSON(raw bool) {
	r.rawJSON = raw
}

// MarshalJSON adds user handles and names while preserving canonical user IDs.
func (r SearchResult) MarshalJSON() ([]byte, error) {
	type output struct {
		Query         string `json:"query"`
		Page          int    `json:"page"`
		PageCount     int    `json:"page_count"`
		NextPage      int    `json:"next_page,omitempty"`
		HasMore       bool   `json:"has_more"`
		ReturnedCount int    `json:"returned_count"`
		Incomplete    bool   `json:"incomplete"`
		Messages      struct {
			Total   int                      `json:"total"`
			Matches []map[string]interface{} `json:"matches"`
		} `json:"messages"`
	}

	result := output{Query: r.Query, Page: r.Page, PageCount: r.PageCount,
		NextPage: r.NextPage, HasMore: r.HasMore, ReturnedCount: len(r.Messages.Matches),
		Incomplete: len(r.Messages.Matches) < r.Messages.Total}
	result.Messages.Total = r.Messages.Total
	result.Messages.Matches = make([]map[string]interface{}, len(r.Messages.Matches))

	for i, match := range r.Messages.Matches {
		entry := map[string]interface{}{
			"type":      match.Type,
			"user":      match.User,
			"username":  match.Username,
			"ts":        match.Timestamp,
			"text":      match.Text,
			"permalink": match.Permalink,
			"channel": map[string]interface{}{
				"id":   match.Channel.ID,
				"name": match.Channel.Name,
			},
		}
		if len(match.Attachments) > 0 {
			entry["attachments"] = match.Attachments
		}
		if len(match.Blocks.BlockSet) > 0 {
			entry["blocks"] = match.Blocks
		}

		if !r.rawJSON {
			delete(entry, "username")
			if match.User != "" {
				entry["user_id"] = match.User
			}
			if username := r.resolvedSearchUsername(match.User); username != "" {
				entry["username"] = username
			}
			if name := r.resolvedSearchDisplayName(match); name != "" {
				entry["display_name"] = name
			}

			if channel, ok := entry["channel"].(map[string]interface{}); ok {
				channel["name"] = r.resolvedSearchChannelRef(match.Channel)
			}
		}

		result.Messages.Matches[i] = entry
	}

	return json.Marshal(result)
}

// Lines implements the output.Printable interface for human-readable search results.
func (r *SearchResult) Lines() []string {
	lines := []string{
		fmt.Sprintf("Search Results for \"%s\" (%d matches)", r.Query, r.Messages.Total),
		"───────────────────────────────────────────────────",
	}
	if len(r.Messages.Matches) < r.Messages.Total {
		lines = append(lines, fmt.Sprintf("Showing %d of %d matches.", len(r.Messages.Matches), r.Messages.Total))
	}
	if r.HasMore {
		lines = append(lines, fmt.Sprintf("More results: use slk search messages with the same query, --limit, and sort options, plus --page %d.", r.NextPage))
	} else if len(r.Messages.Matches) < r.Messages.Total {
		lines = append(lines, "Incomplete results: no further page was provided. Earlier pages may be excluded; narrow the query or read known message permalinks.")
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

		name, username := r.resolvedSearchDisplayName(match), r.resolvedSearchUsername(match.User)
		if name != "" && username != "" && name != strings.TrimPrefix(username, "@") {
			name += " (" + username + ")"
		} else if username != "" {
			name = username
		} else if name == "" {
			name = match.User
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[%s] #%s %s:", ts, channelName, name))
		for _, line := range match.ContentLines() {
			lines = append(lines, "  "+line)
		}
		if match.Permalink != "" {
			lines = append(lines, fmt.Sprintf("  %s", match.Permalink))
		}
	}

	return lines
}

// ContentLines previews search content without fetching each matching message.
// JSON retains structured content; the preview makes that distinction explicit.
func (m SearchMatch) ContentLines() []string {
	var lines []string
	add := func(values ...string) {
		for _, value := range values {
			if value != "" {
				lines = append(lines, value)
			}
		}
	}
	add(m.Text)
	for _, attachment := range m.Attachments {
		add(attachment.Pretext, attachment.AuthorName, attachment.AuthorLink,
			attachment.Title, attachment.TitleLink, attachment.Text)
		for _, field := range attachment.Fields {
			add(strings.TrimPrefix(field.Title+": "+field.Value, ": "))
		}
		if attachment.Fallback != attachment.Text {
			add(attachment.Fallback)
		}
		add(attachment.Footer, attachment.ImageURL, attachment.ThumbURL,
			attachment.FromURL, attachment.OriginalURL)
		lines = append(lines, searchBlockLines(attachment.Blocks)...)
	}
	lines = append(lines, searchBlockLines(m.Blocks)...)
	if len(m.Attachments) > 0 || len(m.Blocks.BlockSet) > 0 {
		add("Text preview; use the default JSON output or the message permalink for full attachment/block structure.")
	}
	if len(lines) == 0 {
		add("[No text content returned by Slack; inspect the message permalink.]")
	}
	return lines
}

func searchBlockLines(blocks slackapi.Blocks) []string {
	var lines []string
	addText := func(text *slackapi.TextBlockObject) {
		if text != nil && text.Text != "" {
			lines = append(lines, text.Text)
		}
	}
	for _, block := range blocks.BlockSet {
		switch block := block.(type) {
		case *slackapi.SectionBlock:
			addText(block.Text)
			for _, field := range block.Fields {
				addText(field)
			}
		case *slackapi.HeaderBlock:
			addText(block.Text)
		default:
			// Keep uncommon and future block types inspectable without a second
			// renderer that has to keep pace with Slack's schema.
			encoded, err := json.Marshal(block)
			if err != nil {
				lines = append(lines, "[Block preview unavailable; inspect the message permalink.]")
				continue
			}
			lines = append(lines, "Block (JSON): "+string(encoded))
		}
	}
	return lines
}

func (r SearchResult) resolvedSearchUsername(userID string) string {
	if userID == "" {
		return ""
	}
	if r.userResolver != nil && r.ctx != nil {
		name := r.userResolver.GetMentionName(r.ctx, userID)
		if name != "" && name != userID {
			return formatSearchUserRef(name)
		}
	}
	return ""
}

func (r SearchResult) resolvedSearchDisplayName(match SearchMatch) string {
	if r.userResolver != nil && r.ctx != nil && match.User != "" {
		name := r.userResolver.GetDisplayName(r.ctx, match.User)
		if name != "" && name != match.User {
			return name
		}
	}
	return match.Username
}

func (r SearchResult) resolvedSearchChannelRef(channel SearchChannel) string {
	name := strings.TrimSpace(channel.Name)
	if name == "" && r.channelResolver != nil && r.ctx != nil && channel.ID != "" {
		resolved := strings.TrimSpace(r.channelResolver.ResolveName(r.ctx, channel.ID))
		if resolved != "" && resolved != channel.ID {
			name = resolved
		}
	}
	if name == "" {
		return channel.ID
	}
	if strings.HasPrefix(name, "#") {
		return name
	}
	return "#" + name
}

func formatSearchUserRef(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "@") {
		return trimmed
	}
	return "@" + trimmed
}
