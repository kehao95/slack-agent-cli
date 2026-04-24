package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/eventstore"
	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type streamFilter struct {
	ChannelID         string
	ConversationTypes map[string]struct{}
	ThreadTS          string
	UserID            string
	ThreadsOnly       bool
	ExcludeSelf       bool
}

func (f streamFilter) Match(event streamEvent) bool {
	if f.ChannelID != "" && event.ChannelID != f.ChannelID {
		return false
	}

	if len(f.ConversationTypes) > 0 {
		if _, ok := f.ConversationTypes[event.ConversationType]; !ok {
			return false
		}
	}

	if f.ThreadTS != "" {
		if event.ThreadTS != f.ThreadTS && event.TS != f.ThreadTS {
			return false
		}
	}

	if f.UserID != "" && event.UserID != f.UserID {
		return false
	}

	if f.ThreadsOnly {
		if !event.IsThreadReply && !event.IsThreadRoot && event.Subtype != "message_replied" && event.Subtype != "thread_broadcast" {
			return false
		}
	}

	if f.ExcludeSelf && event.IsSelf {
		return false
	}

	return true
}

type streamEvent struct {
	Cursor           int64           `json:"cursor,omitempty"`
	ReceivedAt       time.Time       `json:"received_at,omitempty"`
	Kind             string          `json:"kind"`
	EnvelopeID       string          `json:"envelope_id,omitempty"`
	EventID          string          `json:"event_id,omitempty"`
	EventTime        int             `json:"event_time,omitempty"`
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype,omitempty"`
	Channel          string          `json:"channel,omitempty"`
	ChannelID        string          `json:"channel_id,omitempty"`
	ConversationType string          `json:"conversation_type,omitempty"`
	User             string          `json:"user,omitempty"`
	UserID           string          `json:"user_id,omitempty"`
	BotID            string          `json:"bot_id,omitempty"`
	ItemUser         string          `json:"item_user,omitempty"`
	ItemUserID       string          `json:"item_user_id,omitempty"`
	Reaction         string          `json:"reaction,omitempty"`
	TS               string          `json:"ts,omitempty"`
	ThreadTS         string          `json:"thread_ts,omitempty"`
	Text             string          `json:"text,omitempty"`
	IsThreadReply    bool            `json:"is_thread_reply,omitempty"`
	IsThreadRoot     bool            `json:"is_thread_root,omitempty"`
	IsSelf           bool            `json:"is_self,omitempty"`
	Raw              json.RawMessage `json:"raw,omitempty"`
}

type eventNormalizer struct {
	ctx                  context.Context
	channelResolver      streamChannelResolver
	userResolver         streamUserResolver
	conversationProvider streamConversationInfoProvider
	conversationTypeByID map[string]string
	selfUserID           string
	selfBotID            string
}

type streamChannelResolver interface {
	ResolveName(ctx context.Context, channelID string) string
}

type streamUserResolver interface {
	GetMentionName(ctx context.Context, userID string) string
}

type streamConversationInfoProvider interface {
	GetConversationInfo(ctx context.Context, channelID string) (*slackapi.Channel, error)
}

func newEventNormalizer(cmdCtx *CommandContext) *eventNormalizer {
	return &eventNormalizer{
		ctx:                  cmdCtx.Ctx,
		channelResolver:      cmdCtx.ChannelResolver,
		userResolver:         cmdCtx.UserResolver,
		conversationProvider: cmdCtx.Client,
		conversationTypeByID: map[string]string{},
		selfUserID:           strings.TrimSpace(cmdCtx.AuthUserID),
		selfBotID:            strings.TrimSpace(cmdCtx.AuthBotID),
	}
}

func streamEventFromStore(event eventstore.Event) streamEvent {
	return streamEvent{
		Cursor:           event.Cursor,
		ReceivedAt:       event.ReceivedAt,
		Kind:             event.Kind,
		EnvelopeID:       event.EnvelopeID,
		EventID:          event.EventID,
		EventTime:        event.EventTime,
		Type:             event.Type,
		Subtype:          event.Subtype,
		Channel:          event.Channel,
		ChannelID:        event.ChannelID,
		ConversationType: event.ConversationType,
		User:             event.User,
		UserID:           event.UserID,
		BotID:            event.BotID,
		ItemUser:         event.ItemUser,
		ItemUserID:       event.ItemUserID,
		Reaction:         event.Reaction,
		TS:               event.TS,
		ThreadTS:         event.ThreadTS,
		Text:             event.Text,
		IsThreadReply:    event.IsThreadReply,
		IsThreadRoot:     event.IsThreadRoot,
		IsSelf:           event.IsSelf,
		Raw:              event.Raw,
	}
}

func streamEventToStore(event streamEvent) eventstore.Event {
	return eventstore.Event{
		Cursor:           event.Cursor,
		ReceivedAt:       event.ReceivedAt,
		Kind:             event.Kind,
		EnvelopeID:       event.EnvelopeID,
		EventID:          event.EventID,
		EventTime:        event.EventTime,
		Type:             event.Type,
		Subtype:          event.Subtype,
		Channel:          event.Channel,
		ChannelID:        event.ChannelID,
		ConversationType: event.ConversationType,
		User:             event.User,
		UserID:           event.UserID,
		BotID:            event.BotID,
		ItemUser:         event.ItemUser,
		ItemUserID:       event.ItemUserID,
		Reaction:         event.Reaction,
		TS:               event.TS,
		ThreadTS:         event.ThreadTS,
		Text:             event.Text,
		IsThreadReply:    event.IsThreadReply,
		IsThreadRoot:     event.IsThreadRoot,
		IsSelf:           event.IsSelf,
		Raw:              event.Raw,
	}
}

func (n *eventNormalizer) Normalize(eventsAPIEvent slackevents.EventsAPIEvent, req *socketmode.Request, includeRaw bool) (streamEvent, bool, error) {
	event := streamEvent{
		Kind: "slack.event",
	}
	if req != nil {
		event.EnvelopeID = req.EnvelopeID
		event.EventID, event.EventTime = extractEventMetadata(req.Payload)
		if includeRaw {
			event.Raw = append(json.RawMessage(nil), req.Payload...)
		}
	}

	if eventsAPIEvent.Type != slackevents.CallbackEvent {
		event.Type = string(eventsAPIEvent.Type)
		return event, true, nil
	}

	switch inner := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		return n.normalizeMessageEvent(event, eventsAPIEvent.InnerEvent.Type, inner), true, nil
	case *slackevents.ReactionAddedEvent:
		return n.normalizeReactionEvent(
			event,
			eventsAPIEvent.InnerEvent.Type,
			inner.User,
			inner.ItemUser,
			inner.Reaction,
			inner.Item,
		), true, nil
	case *slackevents.ReactionRemovedEvent:
		return n.normalizeReactionEvent(
			event,
			eventsAPIEvent.InnerEvent.Type,
			inner.User,
			inner.ItemUser,
			inner.Reaction,
			inner.Item,
		), true, nil
	case *slackevents.PinAddedEvent:
		return n.normalizePinEvent(event, eventsAPIEvent.InnerEvent.Type, inner.User, inner.Channel, inner.Item), true, nil
	case *slackevents.PinRemovedEvent:
		return n.normalizePinEvent(event, eventsAPIEvent.InnerEvent.Type, inner.User, inner.Channel, inner.Item), true, nil
	default:
		event.Type = eventsAPIEvent.InnerEvent.Type
		return event, true, nil
	}
}

func (n *eventNormalizer) normalizeMessageEvent(base streamEvent, eventType string, evt *slackevents.MessageEvent) streamEvent {
	payload := evt
	if evt.Message != nil {
		payload = evt.Message
	}

	ts := firstNonEmpty(payload.TimeStamp, evt.TimeStamp)
	threadTS := firstNonEmpty(payload.ThreadTimeStamp, evt.ThreadTimeStamp)
	userID := firstNonEmpty(payload.User, evt.User)
	channelID := firstNonEmpty(payload.Channel, evt.Channel)
	subtype := firstNonEmpty(evt.SubType, payload.SubType)
	botID := firstNonEmpty(payload.BotID, evt.BotID)
	conversationType := firstNonEmpty(normalizeConversationType(payload.ChannelType), normalizeConversationType(evt.ChannelType))
	if conversationType == "" {
		conversationType = n.resolveConversationType(channelID)
	}

	base.Type = eventType
	base.Subtype = subtype
	base.ChannelID = channelID
	base.Channel = n.resolveChannelRef(channelID, conversationType)
	base.ConversationType = conversationType
	base.UserID = userID
	base.User = n.resolveUserRef(userID)
	base.BotID = botID
	base.IsSelf = n.isSelf(userID, botID)
	base.TS = ts
	base.ThreadTS = threadTS
	base.Text = firstNonEmpty(payload.Text, evt.Text)
	base.IsThreadReply = threadTS != "" && ts != "" && threadTS != ts
	base.IsThreadRoot = threadTS != "" && ts != "" && threadTS == ts

	return base
}

func (n *eventNormalizer) normalizeReactionEvent(base streamEvent, eventType, userID, itemUserID, reaction string, item slackevents.Item) streamEvent {
	channelID := strings.TrimSpace(item.Channel)
	conversationType := n.resolveConversationType(channelID)
	ts := firstNonEmpty(item.Timestamp, messageTimestamp(item.Message))

	base.Type = eventType
	base.ChannelID = channelID
	base.Channel = n.resolveChannelRef(channelID, conversationType)
	base.ConversationType = conversationType
	base.UserID = userID
	base.User = n.resolveUserRef(userID)
	base.IsSelf = n.isSelf(userID, "")
	base.ItemUserID = itemUserID
	base.ItemUser = n.resolveUserRef(itemUserID)
	base.Reaction = reaction
	base.TS = ts
	base.Text = messageText(item.Message)

	return base
}

func (n *eventNormalizer) normalizePinEvent(base streamEvent, eventType, userID, channelID string, item slackevents.Item) streamEvent {
	channelID = firstNonEmpty(channelID, item.Channel)
	conversationType := n.resolveConversationType(channelID)
	ts := firstNonEmpty(item.Timestamp, messageTimestamp(item.Message))

	base.Type = eventType
	base.ChannelID = channelID
	base.Channel = n.resolveChannelRef(channelID, conversationType)
	base.ConversationType = conversationType
	base.UserID = userID
	base.User = n.resolveUserRef(userID)
	base.IsSelf = n.isSelf(userID, "")
	base.TS = ts
	base.Text = messageText(item.Message)

	return base
}

func (n *eventNormalizer) resolveUserRef(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	if n.userResolver == nil || n.ctx == nil {
		return userID
	}
	resolved := strings.TrimSpace(n.userResolver.GetMentionName(n.ctx, userID))
	if resolved == "" || resolved == userID {
		return userID
	}
	if strings.HasPrefix(resolved, "@") {
		return resolved
	}
	return "@" + resolved
}

func (n *eventNormalizer) isSelf(userID, botID string) bool {
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	return (userID != "" && n.selfUserID != "" && userID == n.selfUserID) ||
		(botID != "" && n.selfBotID != "" && botID == n.selfBotID)
}

func (n *eventNormalizer) resolveChannelRef(channelID, conversationType string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return ""
	}
	if n.channelResolver == nil || n.ctx == nil {
		return channelID
	}
	resolved := strings.TrimSpace(n.channelResolver.ResolveName(n.ctx, channelID))
	if resolved == "" || resolved == channelID {
		return channelID
	}
	if strings.HasPrefix(resolved, "#") || strings.HasPrefix(resolved, "@") {
		return resolved
	}
	switch conversationType {
	case "channel", "private":
		return "#" + resolved
	default:
		return resolved
	}
}

func (n *eventNormalizer) resolveConversationType(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return ""
	}
	if cached := n.conversationTypeByID[channelID]; cached != "" {
		return cached
	}

	switch {
	case strings.HasPrefix(channelID, "C"):
		n.conversationTypeByID[channelID] = "channel"
		return "channel"
	case strings.HasPrefix(channelID, "D"):
		n.conversationTypeByID[channelID] = "dm"
		return "dm"
	case strings.HasPrefix(channelID, "G"):
		if n.conversationProvider != nil && n.ctx != nil {
			info, err := n.conversationProvider.GetConversationInfo(n.ctx, channelID)
			if err == nil && info != nil {
				switch {
				case info.IsMpIM:
					n.conversationTypeByID[channelID] = "mpdm"
					return "mpdm"
				case info.IsPrivate || info.IsGroup:
					n.conversationTypeByID[channelID] = "private"
					return "private"
				}
			}
		}
		n.conversationTypeByID[channelID] = "private"
		return "private"
	default:
		return ""
	}
}

func parseConversationTypes(raw string) (map[string]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	valid := map[string]struct{}{
		"channel":  {},
		"private":  {},
		"dm":       {},
		"mpdm":     {},
		"app_home": {},
	}

	result := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := normalizeConversationType(part)
		if value == "" {
			return nil, fmt.Errorf("invalid conversation type %q: must be one of channel, private, dm, mpdm, app_home", strings.TrimSpace(part))
		}
		if _, ok := valid[value]; !ok {
			return nil, fmt.Errorf("invalid conversation type %q: must be one of channel, private, dm, mpdm, app_home", strings.TrimSpace(part))
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func normalizeConversationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "channel":
		return "channel"
	case "group", "private", "private_channel":
		return "private"
	case "im", "dm":
		return "dm"
	case "mpim", "mim", "mpdm":
		return "mpdm"
	case "app_home", "apphome":
		return "app_home"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func messageTimestamp(message *slackevents.ItemMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.Timestamp)
}

func messageText(message *slackevents.ItemMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.Text)
}

func extractEventMetadata(payload json.RawMessage) (string, int) {
	if len(payload) == 0 {
		return "", 0
	}

	var meta struct {
		EventID   string `json:"event_id"`
		EventTime int    `json:"event_time"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return "", 0
	}
	return meta.EventID, meta.EventTime
}

func formatHumanStreamEvent(event streamEvent) string {
	parts := []string{}

	if event.TS != "" {
		parts = append(parts, "["+formatEventTimestamp(event.TS)+"]")
	} else if event.EventTime != 0 {
		parts = append(parts, "["+time.Unix(int64(event.EventTime), 0).Local().Format("15:04:05")+"]")
	}

	scope := strings.TrimSpace(event.Channel)
	if scope == "" {
		scope = strings.TrimSpace(event.ChannelID)
	}
	if scope != "" {
		parts = append(parts, scope)
	}

	if event.User != "" {
		parts = append(parts, event.User)
	}

	switch event.Type {
	case "message":
		label := "message"
		if event.Subtype != "" {
			label = event.Subtype
		}
		if event.IsThreadReply {
			label += " thread-reply"
		} else if event.IsThreadRoot {
			label += " thread-root"
		}
		body := strings.TrimSpace(event.Text)
		if body == "" {
			body = "(no text)"
		}
		return strings.Join(parts, " ") + ": " + label + " - " + body
	case "reaction_added", "reaction_removed":
		target := strings.TrimSpace(event.ItemUser)
		if target == "" {
			target = strings.TrimSpace(event.ItemUserID)
		}
		body := event.Type
		if event.Reaction != "" {
			body += " :" + event.Reaction + ":"
		}
		if target != "" {
			body += " on " + target
		}
		if event.Text != "" {
			body += " - " + event.Text
		}
		return strings.Join(parts, " ") + ": " + body
	case "pin_added", "pin_removed":
		body := event.Type
		if event.Text != "" {
			body += " - " + event.Text
		}
		return strings.Join(parts, " ") + ": " + body
	default:
		body := event.Type
		if event.Subtype != "" {
			body += "/" + event.Subtype
		}
		return strings.Join(parts, " ") + ": " + body
	}
}

func formatEventTimestamp(ts string) string {
	whole := ts
	if idx := strings.Index(ts, "."); idx >= 0 {
		whole = ts[:idx]
	}
	sec, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return ts
	}
	return time.Unix(sec, 0).Local().Format("15:04:05")
}
