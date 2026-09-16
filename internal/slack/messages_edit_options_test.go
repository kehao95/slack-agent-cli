package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestEditMessageExplicitClearsWithContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("unexpected method: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("text"); got != "retained" {
			t.Fatalf("text = %q, want retained", got)
		}
		for _, field := range []string{"blocks", "attachments"} {
			if r.Form.Get(field) != "[]" {
				t.Errorf("%s = %q, want explicit []", field, r.Form.Get(field))
			}
		}
		if r.Form.Get("metadata") != "{}" {
			t.Fatalf("metadata = %q, want {}", r.Form.Get("metadata"))
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "channel": "C123", "ts": "1.000001", "text": "retained"})
	}))
	defer server.Close()

	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	client.endpoint = server.URL + "/"
	client.rawHTTPClient = server.Client()
	_, err := client.EditMessageWithOptions(context.Background(), "C123", "1.000001", PostMessageOptions{
		Text: "retained", TextSet: true, BlocksSet: true, AttachmentsSet: true, MetadataClear: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditMessageClearOnlyReportsSlackContentRequirement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Fatalf("unexpected method: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if _, exists := r.Form["text"]; exists {
			t.Error("clear-only edit supplied implicit text")
		}
		if r.Form.Get("blocks") != "[]" || r.Form.Get("attachments") != "[]" || r.Form.Get("metadata") != "{}" {
			t.Fatalf("clear fields = blocks %q attachments %q metadata %q", r.Form.Get("blocks"), r.Form.Get("attachments"), r.Form.Get("metadata"))
		}
		writeJSON(t, w, map[string]interface{}{"ok": false, "error": "no_text"})
	}))
	defer server.Close()

	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	client.endpoint = server.URL + "/"
	client.rawHTTPClient = server.Client()
	_, err := client.EditMessageWithOptions(context.Background(), "C123", "1.000001", PostMessageOptions{
		BlocksSet: true, AttachmentsSet: true, MetadataClear: true,
	})
	if err == nil || !strings.Contains(err.Error(), "provide --text or non-empty --blocks") {
		t.Fatalf("error = %v, want actionable content requirement", err)
	}
}
