package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestEditMessageMetadataOnlyPreservesUnspecifiedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("unexpected method: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"text", "blocks", "attachments"} {
			if _, exists := r.Form[field]; exists {
				t.Errorf("metadata-only update supplied %s", field)
			}
		}
		metadata := r.Form.Get("metadata")
		if metadata == "" {
			t.Fatal("metadata-only update omitted metadata")
		}
		var object map[string]interface{}
		if err := json.Unmarshal([]byte(metadata), &object); err != nil {
			t.Fatalf("metadata JSON: %v", err)
		}
		if object["event_type"] != "task_created" {
			t.Fatalf("metadata = %#v", object)
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "channel": "C123", "ts": "1.000001", "text": "unchanged"})
	}))
	defer server.Close()

	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	client.endpoint = server.URL + "/"
	client.rawHTTPClient = server.Client()
	metadata := &slackapi.SlackMetadata{EventType: "task_created", EventPayload: map[string]interface{}{"id": "task-1"}}
	if _, err := client.EditMessageWithOptions(context.Background(), "C123", "1.000001", PostMessageOptions{Metadata: metadata}); err != nil {
		t.Fatal(err)
	}
}

func TestEditMessageMetadataClearPreservesUnspecifiedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("unexpected method: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"text", "blocks", "attachments"} {
			if _, exists := r.Form[field]; exists {
				t.Errorf("metadata-clear update supplied %s", field)
			}
		}
		if got := r.Form.Get("metadata"); got != "{}" {
			t.Fatalf("metadata = %q, want {}", got)
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "channel": "C123", "ts": "1.000001", "text": "unchanged"})
	}))
	defer server.Close()

	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	client.endpoint = server.URL + "/"
	client.rawHTTPClient = server.Client()
	if _, err := client.EditMessageWithOptions(context.Background(), "C123", "1.000001", PostMessageOptions{MetadataClear: true}); err != nil {
		t.Fatal(err)
	}
}
