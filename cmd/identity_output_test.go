package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/eventstore"
	"github.com/slack-go/slack/slackevents"
	"github.com/spf13/cobra"
)

func TestEventIdentitySurvivesStoreWithoutPromotingDisplayNames(t *testing.T) {
	ctx := context.Background()
	store, err := eventstore.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	normalizer := &eventNormalizer{
		ctx: ctx,
		userResolver: testUserResolver{
			names:        map[string]string{"U123": "alice"},
			displayNames: map[string]string{"U123": "Same Name", "U456": "Same Name"},
		},
	}
	for _, userID := range []string{"U123", "U456"} {
		text := "  hi <@U123>\n"
		event := normalizer.normalizeMessageEvent(streamEvent{Kind: "slack.event"}, "message", &slackevents.MessageEvent{
			User: userID, Text: text, Channel: "C123", ChannelType: "channel",
		})
		if event.User != userID || event.UserID != userID || event.DisplayName != "Same Name" || event.Text != text {
			t.Fatalf("identity/text changed: %+v", event)
		}
		if userID == "U456" && event.Username != "" {
			t.Fatalf("display-only user acquired a handle: %+v", event)
		}
		if strings.Contains(formatHumanStreamEvent(event), "@Same Name") {
			t.Fatal("human output presented a display name as a handle")
		}
		if _, err := store.Insert(ctx, streamEventToStore(event)); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.Query(ctx, eventstore.Filter{Limit: 10})
	if err != nil || len(events) != 2 {
		t.Fatalf("query = %v, %v", events, err)
	}
	for _, stored := range events {
		out := streamEventFromStore(stored)
		encoded, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]interface{}
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if fields["user"] != stored.UserID || fields["display_name"] != "Same Name" {
			t.Fatalf("stored identity changed: %s", encoded)
		}
		if stored.UserID == "U123" && fields["username"] != "@alice" {
			t.Fatalf("lost authoritative username: %s", encoded)
		}
		if stored.UserID == "U456" {
			if _, found := fields["username"]; found {
				t.Fatalf("fabricated username: %s", encoded)
			}
		}
	}
}

func TestMessagesNextKeepsEventIdentityMetadata(t *testing.T) {
	for _, username := range []string{"@alice", ""} {
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		event := eventstore.Event{
			Type: "message", UserID: "U123", User: "U123",
			Username: username, DisplayName: "Alice Example", Text: "hi <@U456>",
		}
		if err := printCachedMessageEvent(cmd, event); err != nil {
			t.Fatal(err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["user"] != "U123" || result["user_id"] != "U123" || result["display_name"] != "Alice Example" || result["text"] != event.Text {
			t.Fatalf("messages next lost identity/text: %s", out.Bytes())
		}
		if username != "" && result["username"] != username {
			t.Fatalf("messages next lost username: %s", out.Bytes())
		}
		if _, exists := result["username"]; username == "" && exists {
			t.Fatalf("messages next fabricated username: %s", out.Bytes())
		}
	}
}

func TestReactionIdentityKeepsActorAndTargetSeparate(t *testing.T) {
	normalizer := &eventNormalizer{
		ctx: context.Background(),
		userResolver: testUserResolver{
			names:        map[string]string{"U123": "alice", "U456": "bob"},
			displayNames: map[string]string{"U123": "Same Name", "U456": "Same Name"},
		},
	}
	event := normalizer.normalizeReactionEvent(streamEvent{}, "reaction_added", "U123", "U456", "eyes", slackevents.Item{})
	if event.User != "U123" || event.ItemUser != "U456" || event.Username != "@alice" || event.ItemUsername != "@bob" {
		t.Fatalf("actor/target identity mixed: %+v", event)
	}
	roundTrip := streamEventFromStore(streamEventToStore(event))
	before, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("event store mapping lost metadata: %s -> %s", before, after)
	}
}
