package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestAuthIdentitySeparatesIDAndUsername(t *testing.T) {
	for _, bot := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "bot"}[bot], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/auth.test" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				payload := map[string]interface{}{"ok": true, "user_id": "U123", "user": "alice", "team_id": "T123"}
				if bot {
					payload["bot_id"] = "B123"
				}
				_ = json.NewEncoder(w).Encode(payload)
			}))
			defer server.Close()
			client := New("test-token", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := client.AuthTest(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if result.User != "U123" || result.UserID != "U123" || result.Username != "@alice" || result.IsBot != bot {
				t.Fatalf("incorrect token principal: %+v", result)
			}
		})
	}
}
