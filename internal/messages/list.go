package messages

import (
	"context"
	"encoding/json"
	"errors"
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
	Channel         string
	Limit           int
	Since           string
	Until           string
	Thread          string
	Cursor          string
	All             bool
	PageDelay       time.Duration
	RetryRateLimits bool
	MaxRetries      int
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

// SetRawJSON disables identity enrichment and preserves native Slack fields.
func (r *Result) SetRawJSON(raw bool) {
	r.rawJSON = raw
}

// MarshalJSON adds exact handles and presentation names without replacing IDs.
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

		if !r.rawJSON {
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

	cursor := params.Cursor
	seen := map[string]bool{}
	result := Result{Channel: params.Channel, ThreadTS: params.Thread}
	for {
		msgs, nextCursor, more, err := s.fetchPage(ctx, params, cursor, oldest, latest)
		if err != nil {
			return Result{}, err
		}
		result.Messages = append(result.Messages, msgs...)
		result.HasMore = more
		result.NextCursor = nextCursor
		if !params.All || nextCursor == "" {
			break
		}
		if seen[nextCursor] {
			return Result{}, fmt.Errorf("Slack returned repeated message cursor %q", nextCursor)
		}
		seen[nextCursor] = true
		if err := waitFor(ctx, params.PageDelay); err != nil {
			return Result{}, err
		}
		cursor = nextCursor
	}
	if params.All {
		result.HasMore = false
		result.NextCursor = ""
	}
	return result, nil
}

func (s *Service) fetchPage(ctx context.Context, params Params, cursor, oldest, latest string) ([]slackapi.Message, string, bool, error) {
	maxRetries := params.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	for attempt := 0; ; attempt++ {
		var (
			msgs       []slackapi.Message
			nextCursor string
			more       bool
			err        error
		)
		if params.Thread != "" {
			msgs, nextCursor, more, err = s.fetcher.ListThread(ctx, slack.ThreadParams{
				Channel: params.Channel, Limit: params.Limit, Cursor: cursor,
				Latest: latest, Oldest: oldest, Thread: params.Thread,
			})
		} else {
			msgs, nextCursor, more, err = s.fetcher.ListMessages(ctx, slack.HistoryParams{
				Channel: params.Channel, Limit: params.Limit, Cursor: cursor,
				Latest: latest, Oldest: oldest, Inclusive: false,
			})
		}
		if err == nil {
			return msgs, nextCursor, more, nil
		}

		var rateLimited *slackapi.RateLimitedError
		if !params.RetryRateLimits || attempt >= maxRetries || !errors.As(err, &rateLimited) {
			return nil, "", false, err
		}
		if err := waitFor(ctx, rateLimited.RetryAfter); err != nil {
			return nil, "", false, err
		}
	}
}

func waitFor(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
		msgLine := fmt.Sprintf("[%s] %s: %s", formatTimestamp(msg.Msg.Timestamp), r.displayUser(msg), text)

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
	name, username := r.resolvedDisplayName(msg), r.resolvedUsername(msg)
	if name != "" && username != "" && name != strings.TrimPrefix(username, "@") {
		return name + " (" + username + ")"
	}
	if username != "" {
		return username
	}
	if name != "" {
		return name
	}
	if msg.Msg.User != "" {
		return msg.Msg.User
	}
	return "unknown"
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
	if r.userResolver != nil && r.ctx != nil && msg.Msg.User != "" {
		username := r.userResolver.GetMentionName(r.ctx, msg.Msg.User)
		if username != "" && username != msg.Msg.User {
			return formatUserRef(username)
		}
	}
	return ""
}

func (r Result) resolvedDisplayName(msg slackapi.Message) string {
	if r.userResolver != nil && r.ctx != nil && msg.Msg.User != "" {
		name := r.userResolver.GetDisplayName(r.ctx, msg.Msg.User)
		if name != "" && name != msg.Msg.User {
			return name
		}
	}
	return msg.Username
}

func (r Result) enrichNestedUserReferences(enriched map[string]interface{}) {
	r.enrichResolvedMap(enriched)
}

func (r Result) enrichResolvedMap(value map[string]interface{}) {
	// Native message usernames may be bot aliases. Only a resolved Slack
	// account username can become a normalized @username lookup handle.
	userID, _ := value["user"].(string)
	alias, hasAlias := value["username"].(string)
	if isLikelyUserID(userID) || hasAlias {
		delete(value, "username")
		if !isLikelyUserID(userID) {
			userID = ""
		}
		msg := slackapi.Message{Msg: slackapi.Msg{User: userID, Username: alias}}
		if username := r.resolvedUsername(msg); username != "" {
			value["username"] = username
		}
		if name := r.resolvedDisplayName(msg); name != "" {
			value["display_name"] = name
		}
	}
	for key, raw := range value {
		// Traverse only SDK identity-bearing structures. Metadata payloads,
		// blocks, attachments, and other application data are opaque.
		switch key {
		case "message", "previous_message", "root", "edited", "comment", "initial_comment":
			if nested, ok := raw.(map[string]interface{}); ok {
				r.enrichResolvedMap(nested)
			}
		case "replies", "reactions", "files", "comments":
			if items, ok := raw.([]interface{}); ok {
				for _, item := range items {
					if nested, ok := item.(map[string]interface{}); ok {
						r.enrichResolvedMap(nested)
					}
				}
			}
		}

		switch key {
		case "user", "inviter", "member":
			userID, ok := raw.(string)
			if !ok || !isLikelyUserID(userID) {
				continue
			}
			value[key+"_id"] = userID
		case "users":
			if _, ok := raw.([]interface{}); ok {
				value["user_ids"] = raw
			}
		case "members":
			if _, ok := raw.([]interface{}); ok {
				value["member_ids"] = raw
			}
		case "parent_user_id":
			userID, ok := raw.(string)
			if !ok || !isLikelyUserID(userID) {
				continue
			}
			value["parent_user"] = userID
		}
	}
}

func isLikelyUserID(value string) bool {
	id, _, err := slack.ParseUserReference(value)
	return err == nil && id != "" && id == value
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
			name := r.userResolver.GetMentionName(r.ctx, userID)
			if name != "" && name != userID {
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
