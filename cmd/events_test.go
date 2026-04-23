package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type testChannelResolver struct {
	names map[string]string
}

func (r testChannelResolver) ResolveName(ctx context.Context, channelID string) string {
	if name, ok := r.names[channelID]; ok {
		return name
	}
	return channelID
}

type testUserResolver struct {
	names map[string]string
}

func (r testUserResolver) GetMentionName(ctx context.Context, userID string) string {
	if name, ok := r.names[userID]; ok {
		return name
	}
	return userID
}

type testConversationProvider struct {
	info map[string]*slackapi.Channel
}

func (p testConversationProvider) GetConversationInfo(ctx context.Context, channelID string) (*slackapi.Channel, error) {
	if info, ok := p.info[channelID]; ok {
		return info, nil
	}
	return nil, nil
}

func TestParseConversationTypes(t *testing.T) {
	types, err := parseConversationTypes("channel, dm,private")
	if err != nil {
		t.Fatalf("parseConversationTypes returned error: %v", err)
	}
	if len(types) != 3 {
		t.Fatalf("expected 3 conversation types, got %d", len(types))
	}
	if _, ok := types["dm"]; !ok {
		t.Fatalf("expected dm to be present")
	}
}

func TestParseConversationTypesInvalid(t *testing.T) {
	if _, err := parseConversationTypes("nope"); err == nil {
		t.Fatal("expected invalid conversation type error")
	}
}

func TestEventNormalizerMessageThread(t *testing.T) {
	raw := json.RawMessage(`{"type":"event_callback","event":{"type":"message"}}`)
	normalizer := &eventNormalizer{
		ctx:             context.Background(),
		channelResolver: testChannelResolver{names: map[string]string{"C123": "support"}},
		userResolver:    testUserResolver{names: map[string]string{"U123": "alice"}},
		conversationProvider: testConversationProvider{
			info: map[string]*slackapi.Channel{},
		},
		conversationTypeByID: map[string]string{},
	}

	event, emit, err := normalizer.Normalize(slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Type: "message",
			Data: &slackevents.MessageEvent{
				Type:            "message",
				User:            "U123",
				Text:            "hello thread",
				TimeStamp:       "1705312365.000100",
				ThreadTimeStamp: "1705312365.000000",
				Channel:         "C123",
				ChannelType:     "channel",
			},
		},
	}, &socketmode.Request{EnvelopeID: "env-1", Payload: raw}, true)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if !emit {
		t.Fatal("expected event to emit")
	}
	if event.Channel != "#support" {
		t.Fatalf("expected resolved channel #support, got %q", event.Channel)
	}
	if event.User != "@alice" {
		t.Fatalf("expected resolved user @alice, got %q", event.User)
	}
	if !event.IsThreadReply {
		t.Fatalf("expected thread reply event")
	}
	if string(event.Raw) != string(raw) {
		t.Fatalf("expected raw payload to round-trip")
	}
}

func TestEventNormalizerReactionConversationType(t *testing.T) {
	normalizer := &eventNormalizer{
		ctx:             context.Background(),
		channelResolver: testChannelResolver{names: map[string]string{"G123": "secret"}},
		userResolver:    testUserResolver{names: map[string]string{"U123": "alice", "U456": "bob"}},
		conversationProvider: testConversationProvider{
			info: map[string]*slackapi.Channel{
				"G123": {GroupConversation: slackapi.GroupConversation{Conversation: slackapi.Conversation{IsPrivate: true}}},
			},
		},
		conversationTypeByID: map[string]string{},
	}

	event, emit, err := normalizer.Normalize(slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Type: "reaction_added",
			Data: &slackevents.ReactionAddedEvent{
				Type:     "reaction_added",
				User:     "U123",
				Reaction: "eyes",
				ItemUser: "U456",
				Item: slackevents.Item{
					Channel:   "G123",
					Timestamp: "1705312365.000100",
				},
			},
		},
	}, nil, false)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if !emit {
		t.Fatal("expected event to emit")
	}
	if event.ConversationType != "private" {
		t.Fatalf("expected private conversation type, got %q", event.ConversationType)
	}
	if event.Channel != "#secret" {
		t.Fatalf("expected resolved channel #secret, got %q", event.Channel)
	}
}

func TestStreamFilterMatch(t *testing.T) {
	filter := streamFilter{
		ChannelID: "D123",
		ConversationTypes: map[string]struct{}{
			"dm": {},
		},
		ThreadTS:    "1705312365.000000",
		ThreadsOnly: true,
	}

	if !filter.Match(streamEvent{
		ChannelID:        "D123",
		ConversationType: "dm",
		ThreadTS:         "1705312365.000000",
		TS:               "1705312365.000100",
		IsThreadReply:    true,
	}) {
		t.Fatal("expected event to match filter")
	}

	if filter.Match(streamEvent{
		ChannelID:        "C999",
		ConversationType: "channel",
		ThreadTS:         "1705312365.000000",
		TS:               "1705312365.000100",
		IsThreadReply:    true,
	}) {
		t.Fatal("did not expect mismatched channel to match")
	}
}

func TestFormatHumanStreamEventMessage(t *testing.T) {
	line := formatHumanStreamEvent(streamEvent{
		Type:          "message",
		Channel:       "D123",
		User:          "@alice",
		TS:            "1705312365.000100",
		Text:          "hello there",
		IsThreadReply: true,
	})

	if !strings.Contains(line, "D123") {
		t.Fatalf("expected channel in human output, got %q", line)
	}
	if !strings.Contains(line, "@alice") {
		t.Fatalf("expected user in human output, got %q", line)
	}
	if !strings.Contains(line, "thread-reply") {
		t.Fatalf("expected thread indicator in human output, got %q", line)
	}
	if !strings.Contains(line, "hello there") {
		t.Fatalf("expected text in human output, got %q", line)
	}
}

func TestFormatHumanStreamEventReaction(t *testing.T) {
	line := formatHumanStreamEvent(streamEvent{
		Type:     "reaction_added",
		Channel:  "#general",
		User:     "@alice",
		Reaction: "eyes",
		ItemUser: "@bob",
		Text:     "check this out",
	})

	if !strings.Contains(line, "reaction_added") {
		t.Fatalf("expected reaction label in human output, got %q", line)
	}
	if !strings.Contains(line, ":eyes:") {
		t.Fatalf("expected emoji in human output, got %q", line)
	}
	if !strings.Contains(line, "@bob") {
		t.Fatalf("expected target user in human output, got %q", line)
	}
}
