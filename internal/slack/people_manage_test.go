package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetUserProfileFieldsSendsOnlyExplicitFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.profile.set" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("user") != "U123" {
			t.Fatalf("unexpected user: %q", r.Form.Get("user"))
		}
		var profile map[string]interface{}
		if err := json.Unmarshal([]byte(r.Form.Get("profile")), &profile); err != nil {
			t.Fatalf("decode profile: %v", err)
		}
		if profile["title"] != "Engineer" {
			t.Fatalf("unexpected profile payload: %#v", profile)
		}
		for _, omitted := range []string{"real_name", "display_name", "status_text"} {
			if _, present := profile[omitted]; present {
				t.Fatalf("profile unexpectedly included %q: %#v", omitted, profile)
			}
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "profile": map[string]string{"title": "Engineer"}})
	}))
	defer server.Close()

	client := &APIClient{token: "xoxp-test", endpoint: server.URL, rawHTTPClient: server.Client()}
	result, err := client.SetUserProfileFields(context.Background(), "U123", map[string]interface{}{"title": "Engineer"})
	if err != nil {
		t.Fatalf("SetUserProfileFields: %v", err)
	}
	if result == nil || !result.OK || result.Profile.Title != "Engineer" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.TrimSpace(result.Profile.RealName) != "" {
		t.Fatalf("unexpected omitted profile value: %#v", result.Profile)
	}
}
