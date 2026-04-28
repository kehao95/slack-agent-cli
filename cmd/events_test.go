package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/cobra"
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

func TestParseEventTypes(t *testing.T) {
	types, err := parseEventTypes("message, reaction_added,member_joined_channel")
	if err != nil {
		t.Fatalf("parseEventTypes returned error: %v", err)
	}
	if len(types) != 3 {
		t.Fatalf("expected 3 event types, got %d", len(types))
	}
	if _, ok := types["message"]; !ok {
		t.Fatalf("expected message to be present")
	}
	if _, ok := types["reaction_added"]; !ok {
		t.Fatalf("expected reaction_added to be present")
	}
	if _, ok := types["member_joined_channel"]; !ok {
		t.Fatalf("expected member_joined_channel to be present")
	}
}

func TestParseEventTypesInvalid(t *testing.T) {
	if _, err := parseEventTypes("message, ,reaction_added"); err == nil {
		t.Fatal("expected invalid event type error")
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
		Type:             "message",
		ChannelID:        "D123",
		ConversationType: "dm",
		ThreadTS:         "1705312365.000000",
		TS:               "1705312365.000100",
		IsThreadReply:    true,
	}) {
		t.Fatal("expected event to match filter")
	}

	if filter.Match(streamEvent{
		Type:             "message",
		ChannelID:        "C999",
		ConversationType: "channel",
		ThreadTS:         "1705312365.000000",
		TS:               "1705312365.000100",
		IsThreadReply:    true,
	}) {
		t.Fatal("did not expect mismatched channel to match")
	}
}

func TestStreamFilterEventTypes(t *testing.T) {
	filter := streamFilter{
		EventTypes: map[string]struct{}{
			"message": {},
		},
	}

	if !filter.Match(streamEvent{Type: "message"}) {
		t.Fatal("expected message event to match event-type filter")
	}

	if filter.Match(streamEvent{Type: "reaction_added"}) {
		t.Fatal("did not expect non-matching event type to match filter")
	}

	if !(streamFilter{}).Match(streamEvent{Type: "reaction_added"}) {
		t.Fatal("expected event to match when event-type filter is omitted")
	}
}

func TestBuildEventsStreamFilterRejectsThreadsOnlyWithoutMessageEventType(t *testing.T) {
	cmd := &cobra.Command{Use: "stream"}
	addEventsStreamFlags(cmd)
	if err := cmd.Flags().Set("threads-only", "true"); err != nil {
		t.Fatalf("set threads-only: %v", err)
	}
	if err := cmd.Flags().Set("event-type", "reaction_added"); err != nil {
		t.Fatalf("set event-type: %v", err)
	}

	_, err := buildEventsStreamFilter(cmd, nil)
	if err == nil {
		t.Fatal("expected threads-only with non-message event types to fail")
	}
	if !strings.Contains(err.Error(), "--threads-only only applies to message events") {
		t.Fatalf("expected threads-only validation error, got %v", err)
	}
}

func TestBuildEventsStreamFilterAllowsThreadsOnlyWhenMessageIncluded(t *testing.T) {
	cmd := &cobra.Command{Use: "stream"}
	addEventsStreamFlags(cmd)
	if err := cmd.Flags().Set("threads-only", "true"); err != nil {
		t.Fatalf("set threads-only: %v", err)
	}
	if err := cmd.Flags().Set("event-type", "message,reaction_added"); err != nil {
		t.Fatalf("set event-type: %v", err)
	}

	filter, err := buildEventsStreamFilter(cmd, nil)
	if err != nil {
		t.Fatalf("buildEventsStreamFilter returned error: %v", err)
	}
	if _, ok := filter.EventTypes["message"]; !ok {
		t.Fatalf("expected message to be present in filter")
	}
	if _, ok := filter.EventTypes["reaction_added"]; !ok {
		t.Fatalf("expected reaction_added to be present in filter")
	}
}

func TestStreamFilterThreadsOnlyAllowsExplicitNonMessageEventTypes(t *testing.T) {
	filter := streamFilter{
		EventTypes: map[string]struct{}{
			"message":        {},
			"reaction_added": {},
		},
		ThreadsOnly: true,
	}

	if !filter.Match(streamEvent{Type: "reaction_added"}) {
		t.Fatal("expected explicitly requested non-message event type to pass threads-only filter")
	}

	if !filter.Match(streamEvent{Type: "message", IsThreadReply: true}) {
		t.Fatal("expected thread reply message to pass threads-only filter")
	}

	if filter.Match(streamEvent{Type: "message", TS: "1705312365.000100"}) {
		t.Fatal("did not expect non-thread message to pass threads-only filter")
	}
}

func TestStreamFilterThreadsOnlyWithoutEventTypesRejectsNonMessageEvents(t *testing.T) {
	filter := streamFilter{ThreadsOnly: true}
	if filter.Match(streamEvent{Type: "reaction_added"}) {
		t.Fatal("did not expect non-message event to pass threads-only filter without explicit event types")
	}
}

func TestRunEventsStreamRejectsThreadsOnlyWithoutMessageBeforeConfig(t *testing.T) {
	cmd := &cobra.Command{Use: "stream"}
	addEventsStreamFlags(cmd)
	if err := cmd.Flags().Set("threads-only", "true"); err != nil {
		t.Fatalf("set threads-only: %v", err)
	}
	if err := cmd.Flags().Set("event-type", "reaction_added"); err != nil {
		t.Fatalf("set event-type: %v", err)
	}

	err := runEventsStream(cmd, nil)
	if err == nil {
		t.Fatal("expected invalid event-type/threads-only combination to fail")
	}
	if !strings.Contains(err.Error(), "--threads-only only applies to message events") {
		t.Fatalf("expected threads-only validation error, got %v", err)
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

type mockFileAppenderOpener struct {
	openCount  int
	closeCount int
	buffer     bytes.Buffer
}

func (o *mockFileAppenderOpener) Open(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	o.openCount++
	return &mockWriteCloser{
		buffer: &o.buffer,
		onClose: func() {
			o.closeCount++
		},
	}, nil
}

type mockWriteCloser struct {
	buffer  *bytes.Buffer
	onClose func()
}

func (w *mockWriteCloser) Write(p []byte) (int, error) {
	return w.buffer.Write(p)
}

func (w *mockWriteCloser) Close() error {
	if w.onClose != nil {
		w.onClose()
	}
	return nil
}

func TestAppendFileLineSinkReopensFilePerWrite(t *testing.T) {
	opener := &mockFileAppenderOpener{}
	sink := appendFileLineSink{
		path:   "events.log",
		opener: opener,
	}

	if err := sink.WriteLine([]byte("first")); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := sink.WriteLine([]byte("second")); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	if opener.openCount != 2 {
		t.Fatalf("expected 2 file opens, got %d", opener.openCount)
	}
	if opener.closeCount != 2 {
		t.Fatalf("expected 2 file closes, got %d", opener.closeCount)
	}
	if got := opener.buffer.String(); got != "first\nsecond\n" {
		t.Fatalf("unexpected file contents %q", got)
	}
}

func TestNewEventsStreamSinkWritesToStdoutAndFile(t *testing.T) {
	cmd := &cobra.Command{Use: "stream"}
	addEventsStreamFlags(cmd)

	filePath := filepath.Join(t.TempDir(), "events.ndjson")
	if err := cmd.Flags().Set("file", filePath); err != nil {
		t.Fatalf("set file flag: %v", err)
	}

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	sink, err := newEventsStreamSink(cmd)
	if err != nil {
		t.Fatalf("newEventsStreamSink returned error: %v", err)
	}

	if err := sink.WriteLine([]byte(`{"type":"message"}`)); err != nil {
		t.Fatalf("WriteLine returned error: %v", err)
	}

	if got := stdout.String(); got != "{\"type\":\"message\"}\n" {
		t.Fatalf("unexpected stdout output %q", got)
	}

	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(contents); got != "{\"type\":\"message\"}\n" {
		t.Fatalf("unexpected file output %q", got)
	}
}
