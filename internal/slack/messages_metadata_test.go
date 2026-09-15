package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/policy"
	slackapi "github.com/slack-go/slack"
)

func TestConversationReadsForwardIncludeAllMetadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		enabled bool
		thread  bool
	}{
		{name: "history disabled", path: "/conversations.history"},
		{name: "history enabled", path: "/conversations.history", enabled: true},
		{name: "replies disabled", path: "/conversations.replies", thread: true},
		{name: "replies enabled", path: "/conversations.replies", enabled: true, thread: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Fatalf("path = %q, want %q", r.URL.Path, tc.path)
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				want := "0"
				if tc.enabled {
					want = "1"
				}
				if got := r.Form.Get("include_all_metadata"); got != want {
					t.Fatalf("include_all_metadata = %q, want %q", got, want)
				}
				writeJSON(t, w, map[string]interface{}{
					"ok": true,
					"messages": []map[string]interface{}{{
						"type": "message", "user": "U1", "text": "trace",
						"metadata": map[string]interface{}{
							"event_type": "trace.complete",
							"event_payload": map[string]interface{}{
								"trace_id": "trace-123",
								"nested":   map[string]interface{}{"unknown": []interface{}{true, float64(7)}},
							},
						},
					}},
				})
			}))
			defer server.Close()

			client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			if tc.thread {
				messages, _, _, err := client.ListThreadReplies(context.Background(), ThreadParams{
					Channel: "C1", Thread: "1705312365.000100", IncludeAllMetadata: tc.enabled,
				})
				if err != nil {
					t.Fatal(err)
				}
				assertDecodedMessageMetadata(t, messages)
				return
			}
			response, err := client.ListConversationsHistory(context.Background(), HistoryParams{
				Channel: "C1", IncludeAllMetadata: tc.enabled,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertDecodedMessageMetadata(t, response.Messages)
		})
	}
}

func TestConversationReadsWithMetadataAreAllowedInReadOnlyMode(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		if (r.URL.Path != "/conversations.history" && r.URL.Path != "/conversations.replies") || r.FormValue("include_all_metadata") != "1" {
			t.Fatalf("unexpected request %s: %s", r.URL.Path, r.Form.Encode())
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "messages": []interface{}{}})
	}))
	defer server.Close()

	client := New("xoxb-test", slackapi.OptionAPIURL(server.URL+"/"))
	if _, err := client.ListConversationsHistory(context.Background(), HistoryParams{
		Channel: "C1", IncludeAllMetadata: true,
	}); err != nil {
		t.Fatalf("read-only metadata request: %v", err)
	}
	if _, _, _, err := client.ListThreadReplies(context.Background(), ThreadParams{
		Channel: "C1", Thread: "1705312365.000100", IncludeAllMetadata: true,
	}); err != nil {
		t.Fatalf("read-only replies metadata request: %v", err)
	}
	if requests["/conversations.history"] != 1 || requests["/conversations.replies"] != 1 {
		t.Fatalf("requests = %#v, want one request to each endpoint", requests)
	}
}

func assertDecodedMessageMetadata(t *testing.T, messages []slackapi.Message) {
	t.Helper()
	if len(messages) != 1 {
		t.Fatalf("messages = %#v", messages)
	}
	metadata := messages[0].Metadata
	if metadata.EventType != "trace.complete" || metadata.EventPayload["trace_id"] != "trace-123" {
		t.Fatalf("metadata = %#v", metadata)
	}
	nested, ok := metadata.EventPayload["nested"].(map[string]interface{})
	if !ok || len(nested) != 1 {
		t.Fatalf("nested metadata payload = %#v", metadata.EventPayload)
	}
}
