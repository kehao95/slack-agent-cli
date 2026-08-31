package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestExtendedMessageLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat.getPermalink":
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C1","permalink":"https://example.slack.com/archives/C1/p1"}`))
		case "/chat.postEphemeral":
			if r.Form.Get("user") != "U1" || r.Form.Get("text") != "hello" {
				t.Fatalf("unexpected ephemeral form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true,"message_ts":"1.1"}`))
		case "/chat.scheduleMessage":
			if r.Form.Get("post_at") != "1700000060" {
				t.Fatalf("unexpected schedule form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C1","scheduled_message_id":"Q1"}`))
		case "/chat.scheduledMessages.list":
			_, _ = w.Write([]byte(`{"ok":true,"scheduled_messages":[{"id":"Q1","channel_id":"C1","post_at":1700000060,"text":"later"}],"response_metadata":{"next_cursor":"next"}}`))
		case "/chat.deleteScheduledMessage":
			if r.Form.Get("scheduled_message_id") != "Q1" {
				t.Fatalf("unexpected delete form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/chat.startStream":
			if r.Form.Get("thread_ts") != "1.0" || r.Form.Get("recipient_user_id") != "U1" {
				t.Fatalf("unexpected start stream form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C1","ts":"2.0"}`))
		case "/chat.appendStream":
			if r.Form.Get("ts") != "2.0" || r.Form.Get("markdown_text") != "chunk" {
				t.Fatalf("unexpected append stream form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C1","ts":"2.0"}`))
		case "/chat.stopStream":
			if r.Form.Get("ts") != "2.0" || r.Form.Get("markdown_text") != "final" {
				t.Fatalf("unexpected stop stream form: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":"C1","ts":"2.0"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := New("xoxb-test", slackapi.OptionAPIURL(server.URL+"/"))
	ctx := context.Background()

	permalink, err := client.GetMessagePermalink(ctx, "C1", "1.0")
	if err != nil || permalink.Permalink == "" {
		t.Fatalf("GetMessagePermalink: result=%+v err=%v", permalink, err)
	}
	ephemeral, err := client.PostEphemeralMessage(ctx, "C1", "U1", PostMessageOptions{Text: "hello"})
	if err != nil || ephemeral.Timestamp != "1.1" {
		t.Fatalf("PostEphemeralMessage: result=%+v err=%v", ephemeral, err)
	}
	scheduled, err := client.ScheduleMessage(ctx, "C1", "1700000060", PostMessageOptions{Text: "later"})
	if err != nil || scheduled.ScheduledMessageID != "Q1" {
		t.Fatalf("ScheduleMessage: result=%+v err=%v", scheduled, err)
	}
	listed, err := client.ListScheduledMessages(ctx, "C1", "", "", "", 100)
	if err != nil || len(listed.Messages) != 1 || listed.NextCursor != "next" {
		t.Fatalf("ListScheduledMessages: result=%+v err=%v", listed, err)
	}
	deleted, err := client.DeleteScheduledMessage(ctx, "C1", "Q1", false)
	if err != nil || !deleted.OK {
		t.Fatalf("DeleteScheduledMessage: result=%+v err=%v", deleted, err)
	}
	started, err := client.StartMessageStream(ctx, "C1", StreamStartOptions{ThreadTS: "1.0", RecipientUserID: "U1"})
	if err != nil || started.Timestamp != "2.0" {
		t.Fatalf("StartMessageStream: result=%+v err=%v", started, err)
	}
	appended, err := client.AppendMessageStream(ctx, "C1", "2.0", "chunk")
	if err != nil || appended.Action != "appended" {
		t.Fatalf("AppendMessageStream: result=%+v err=%v", appended, err)
	}
	stopped, err := client.StopMessageStream(ctx, "C1", "2.0", StreamUpdateOptions{Markdown: "final"})
	if err != nil || stopped.Action != "stopped" {
		t.Fatalf("StopMessageStream: result=%+v err=%v", stopped, err)
	}
}
