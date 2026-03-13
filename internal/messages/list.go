package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/slack"
)

// Service coordinates message list operations.
type Fetcher interface {
	ListMessages(context.Context, slack.HistoryParams) ([]slackapi.Message, string, bool, error)
	ListThread(context.Context, slack.ThreadParams) ([]slackapi.Message, string, bool, error)
}

// UserResolver resolves user IDs to display names.
type UserResolver interface {
	GetDisplayName(ctx context.Context, userID string) string
	GetMentionName(ctx context.Context, userID string) string
}

// UserGroupResolver resolves usergroup IDs to handles.
type UserGroupResolver interface {
	GetHandle(ctx context.Context, groupID string) string
}

// Service coordinates message list operations.
type Service struct {
	fetcher Fetcher
}

// NewService constructs a Service.
func NewService(fetcher Fetcher) *Service {
	return &Service{fetcher: fetcher}
}

// Params describes input for List.
type Params struct {
	Channel string
	Limit   int
	Since   string
	Until   string
	Thread  string
	Cursor  string
}

// Result represents list output.
type Result struct {
	Channel           string             `json:"channel"`
	ChannelName       string             `json:"channel_name,omitempty"`
	ThreadTS          string             `json:"thread_ts,omitempty"`
	Messages          []slackapi.Message `json:"messages"`
	HasMore           bool               `json:"has_more"`
	NextCursor        string             `json:"next_cursor"`
	userResolver      UserResolver       `json:"-"`
	userGroupResolver UserGroupResolver  `json:"-"`
	ctx               context.Context    `json:"-"`
	rawJSON           bool               `json:"-"`
}

// SetUserResolver sets the user resolver for human-readable output.
func (r *Result) SetUserResolver(ctx context.Context, resolver UserResolver) {
	r.ctx = ctx
	r.userResolver = resolver
}

// SetUserGroupResolver sets the usergroup resolver for human-readable output.
func (r *Result) SetUserGroupResolver(ctx context.Context, resolver UserGroupResolver) {
	r.ctx = ctx
	r.userGroupResolver = resolver
}

// SetRawJSON controls whether JSON output should preserve raw Slack IDs.
func (r *Result) SetRawJSON(raw bool) {
	r.rawJSON = raw
}

// MarshalJSON enriches the JSON output with resolved usernames for each message.
func (r Result) MarshalJSON() ([]byte, error) {
	type output struct {
		Channel     string                   `json:"channel"`
		ChannelID   string                   `json:"channel_id,omitempty"`
		ChannelName string                   `json:"channel_name,omitempty"`
		ThreadTS    string                   `json:"thread_ts,omitempty"`
		Messages    []map[string]interface{} `json:"messages"`
		HasMore     bool                     `json:"has_more"`
		NextCursor  string                   `json:"next_cursor"`
	}

	channelValue := r.Channel
	channelID := ""
	if !r.rawJSON {
		channelValue = r.resolvedChannelRef()
		if channelValue != r.Channel {
			channelID = r.Channel
		}
	}

	outputValue := output{
		Channel:     channelValue,
		ChannelID:   channelID,
		ChannelName: r.ChannelName,
		ThreadTS:    r.ThreadTS,
		HasMore:     r.HasMore,
		NextCursor:  r.NextCursor,
		Messages:    make([]map[string]interface{}, len(r.Messages)),
	}

	for i, msg := range r.Messages {
		encoded, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}

		var enriched map[string]interface{}
		if err := json.Unmarshal(encoded, &enriched); err != nil {
			return nil, err
		}

		if username := r.resolvedUsername(msg); username != "" {
			enriched["username"] = username
		}

		if !r.rawJSON {
			if userID := msg.Msg.User; userID != "" {
				if resolvedUser := r.resolvedUserRef(msg); resolvedUser != "" {
					enriched["user_id"] = userID
					enriched["user"] = resolvedUser
				}
			} else if msg.Username != "" {
				enriched["user"] = formatUserRef(msg.Username)
			}

			r.enrichNestedUserReferences(enriched)
		}

		outputValue.Messages[i] = enriched
	}

	return json.Marshal(outputValue)
}

// List retrieves channel or thread history.
func (s *Service) List(ctx context.Context, params Params) (Result, error) {
	if params.Channel == "" {
		return Result{}, fmt.Errorf("channel is required")
	}
	oldest, latest, err := slack.ParseTimeRange(params.Since, params.Until)
	if err != nil {
		return Result{}, err
	}
	if params.Thread != "" {
		msgs, cursor, more, err := s.fetcher.ListThread(ctx, slack.ThreadParams{
			Channel: params.Channel,
			Limit:   params.Limit,
			Latest:  latest,
			Oldest:  oldest,
			Thread:  params.Thread,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{Channel: params.Channel, ThreadTS: params.Thread, Messages: msgs, HasMore: more, NextCursor: cursor}, nil
	}
	msgs, cursor, more, err := s.fetcher.ListMessages(ctx, slack.HistoryParams{
		Channel:   params.Channel,
		Limit:     params.Limit,
		Cursor:    params.Cursor,
		Latest:    latest,
		Oldest:    oldest,
		Inclusive: false,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Channel: params.Channel, Messages: msgs, HasMore: more, NextCursor: cursor}, nil
}

// Lines returns human-readable lines for Result.
func (r Result) Lines() []string {
	// Use channel name if available, otherwise channel ID
	channelDisplay := r.ChannelName
	if channelDisplay == "" {
		channelDisplay = r.Channel
	}

	var title string
	if r.ThreadTS != "" {
		title = fmt.Sprintf("#%s - Thread %s - %d messages", strings.TrimPrefix(channelDisplay, "#"), r.ThreadTS, len(r.Messages))
	} else {
		title = fmt.Sprintf("#%s - %d messages", strings.TrimPrefix(channelDisplay, "#"), len(r.Messages))
	}

	lines := []string{title, strings.Repeat("-", len(title))}
	for _, msg := range r.Messages {
		// Resolve user mentions in the message text
		text := r.resolveUserMentions(msg.Msg.Text)
		msgLine := fmt.Sprintf("[%s] @%s: %s", formatTimestamp(msg.Msg.Timestamp), r.displayUser(msg), text)

		// Add thread indicator if message has replies (and we're not already in a thread view)
		if msg.ReplyCount > 0 && r.ThreadTS == "" {
			threadInfo := fmt.Sprintf(" [thread: %d replies, ts: %s]", msg.ReplyCount, msg.ThreadTimestamp)
			msgLine += threadInfo
		}

		lines = append(lines, msgLine)
	}
	if r.NextCursor != "" {
		lines = append(lines, fmt.Sprintf("Next cursor: %s", r.NextCursor))
	}
	return lines
}

func (r Result) displayUser(msg slackapi.Message) string {
	// If we have a username already, use it
	if msg.Username != "" {
		return msg.Username
	}

	userID := msg.Msg.User
	if userID == "" {
		return "unknown"
	}

	// Try to resolve using user resolver
	if r.userResolver != nil && r.ctx != nil {
		name := r.userResolver.GetDisplayName(r.ctx, userID)
		if name != userID { // Only use if actually resolved
			return name
		}
	}

	return userID
}

func (r Result) resolvedChannelRef() string {
	name := strings.TrimSpace(r.ChannelName)
	if name == "" || name == r.Channel {
		return r.Channel
	}
	if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "@") {
		return name
	}
	if strings.HasPrefix(r.Channel, "C") || strings.HasPrefix(r.Channel, "G") {
		return "#" + name
	}
	return name
}

func (r Result) resolvedUsername(msg slackapi.Message) string {
	if msg.Username != "" {
		return msg.Username
	}
	if r.userResolver != nil && r.ctx != nil && msg.Msg.User != "" {
		username := r.userResolver.GetDisplayName(r.ctx, msg.Msg.User)
		if username != msg.Msg.User {
			return username
		}
	}
	return ""
}

func (r Result) resolvedUserRef(msg slackapi.Message) string {
	if msg.Msg.User == "" {
		if msg.Username == "" {
			return ""
		}
		return formatUserRef(msg.Username)
	}
	return r.resolvedUserRefByID(msg.Msg.User)
}

func (r Result) resolvedUserRefByID(userID string) string {
	if r.userResolver != nil && r.ctx != nil {
		name := r.userResolver.GetMentionName(r.ctx, userID)
		if name != "" && name != userID {
			return formatUserRef(name)
		}
	}
	return userID
}

func (r Result) enrichNestedUserReferences(enriched map[string]interface{}) {
	r.enrichResolvedMap(enriched)
}

func (r Result) enrichResolvedMap(value map[string]interface{}) {
	for key, raw := range value {
		switch typed := raw.(type) {
		case map[string]interface{}:
			r.enrichResolvedMap(typed)
		case []interface{}:
			for _, item := range typed {
				if itemMap, ok := item.(map[string]interface{}); ok {
					r.enrichResolvedMap(itemMap)
				}
			}
		}

		switch key {
		case "user", "inviter":
			userID, ok := raw.(string)
			if !ok || !isLikelyUserID(userID) {
				continue
			}
			resolvedUser := r.resolvedUserRefByID(userID)
			if resolvedUser != "" && resolvedUser != userID {
				value[key+"_id"] = userID
				value[key] = resolvedUser
			}
		case "users":
			resolved, changed := r.resolveUserList(raw)
			if changed {
				value["user_ids"] = raw
				value["users"] = resolved
			}
		case "members":
			resolved, changed := r.resolveUserList(raw)
			if changed {
				value["member_ids"] = raw
				value["members"] = resolved
			}
		case "parent_user_id":
			userID, ok := raw.(string)
			if !ok || !isLikelyUserID(userID) {
				continue
			}
			resolvedUser := r.resolvedUserRefByID(userID)
			if resolvedUser != "" && resolvedUser != userID {
				value["parent_user"] = resolvedUser
			}
		}
	}
}

func (r Result) resolveUserList(raw interface{}) ([]interface{}, bool) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil, false
	}

	resolved := make([]interface{}, 0, len(items))
	changed := false
	for _, item := range items {
		userID, ok := item.(string)
		if !ok || !isLikelyUserID(userID) {
			resolved = append(resolved, item)
			continue
		}

		resolvedUser := r.resolvedUserRefByID(userID)
		if resolvedUser != userID {
			changed = true
		}
		resolved = append(resolved, resolvedUser)
	}

	return resolved, changed
}

func isLikelyUserID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "@") {
		return false
	}
	if trimmed != strings.ToUpper(trimmed) {
		return false
	}
	return strings.HasPrefix(trimmed, "U") || strings.HasPrefix(trimmed, "W")
}

func formatUserRef(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "@") {
		return trimmed
	}
	return "@" + trimmed
}

// resolveUserMentions replaces <@USERID> and <!subteam^GROUPID> mentions with @username/@grouphandle in message text.
func (r Result) resolveUserMentions(text string) string {
	// Match user mentions like <@U06D82H8QUW>
	if r.userResolver != nil && r.ctx != nil {
		userMentionRegex := regexp.MustCompile(`<@([A-Z0-9]+)>`)
		text = userMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
			// Extract user ID from <@USERID>
			userID := match[2 : len(match)-1] // Remove <@ and >

			// Try to resolve the user ID
			name := r.userResolver.GetDisplayName(r.ctx, userID)
			if name != userID {
				return "@" + name
			}

			// If resolution failed, keep the original format
			return match
		})
	}

	// Match usergroup mentions like <!subteam^S06EQF4UV5M>
	if r.userGroupResolver != nil && r.ctx != nil {
		usergroupMentionRegex := regexp.MustCompile(`<!subteam\^([A-Z0-9]+)(?:\|[^>]+)?>`)
		text = usergroupMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
			// Extract group ID from <!subteam^GROUPID> or <!subteam^GROUPID|name>
			parts := regexp.MustCompile(`<!subteam\^([A-Z0-9]+)`).FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			groupID := parts[1]

			// Try to resolve the group ID
			handle := r.userGroupResolver.GetHandle(r.ctx, groupID)
			if handle != groupID {
				return "@" + handle
			}

			// If resolution failed, keep the original format
			return match
		})
	}

	return text
}

// formatTimestamp converts a Slack timestamp (e.g., "1769710907.130119") to human-readable format.
func formatTimestamp(ts string) string {
	// Slack timestamps are Unix epoch seconds with microseconds after the dot
	parts := strings.Split(ts, ".")
	if len(parts) == 0 {
		return ts
	}

	secs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return ts
	}

	t := time.Unix(secs, 0)
	now := time.Now()

	// If same day, show only time
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}

	// If same year, show month/day and time
	if t.Year() == now.Year() {
		return t.Format("Jan 02 15:04")
	}

	// Otherwise show full date
	return t.Format("2006-01-02 15:04")
}
