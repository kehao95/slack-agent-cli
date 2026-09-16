package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

// This regression source is intentionally unexecuted in the implementation
// phase. It checks the wire distinction between omission and explicit clearing.
func TestEditMessageExplicitClearsDoNotSupplyText(t *testing.T) {
	for _, clearMetadata := range []bool{false, true} {
		t.Run(map[bool]string{false: "sdk-arrays", true: "metadata-and-arrays"}[clearMetadata], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat.update" {
					t.Errorf("unexpected method: %s", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				for _, field := range []string{"blocks", "attachments"} {
					if r.Form.Get(field) != "[]" {
						t.Errorf("%s = %q, want explicit []", field, r.Form.Get(field))
					}
				}
				if _, exists := r.Form["text"]; exists {
					t.Error("unspecified text was supplied")
				}
				if clearMetadata && r.Form.Get("metadata") != "{}" {
					t.Errorf("metadata = %q, want {}", r.Form.Get("metadata"))
				}
				if !clearMetadata {
					if _, exists := r.Form["metadata"]; exists {
						t.Error("unspecified metadata was supplied")
					}
				}
				writeJSON(t, w, map[string]interface{}{"ok": true, "channel": "C123", "ts": "1.000001", "text": "retained"})
			}))
			defer server.Close()
			client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			client.endpoint = server.URL + "/"
			client.rawHTTPClient = server.Client()
			_, err := client.EditMessageWithOptions(context.Background(), "C123", "1.000001", PostMessageOptions{
				BlocksSet: true, AttachmentsSet: true, MetadataClear: clearMetadata,
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
