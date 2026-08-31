package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestResolveUserReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U123","name":"alice","real_name":"Alice Example","profile":{"display_name":"Ali","email":"alice@example.com"}}],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))

	for _, reference := range []string{"@alice", "Ali", "Alice Example", "alice@example.com", "U123", "@U123", "<@U123>"} {
		t.Run(reference, func(t *testing.T) {
			id, err := client.ResolveUserReference(context.Background(), reference)
			if err != nil {
				t.Fatalf("ResolveUserReference(%q): %v", reference, err)
			}
			if id != "U123" {
				t.Fatalf("ResolveUserReference(%q) = %q", reference, id)
			}
		})
	}
}

func TestResolveUserReferenceAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U1","name":"same"},{"id":"U2","name":"same"}]}`))
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	_, err := client.ResolveUserReference(context.Background(), "@same")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestResolveUserReferencesFetchesOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"members":[{"id":"U1","name":"alice"},{"id":"U2","name":"bob"}]}`))
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	resolved, err := client.ResolveUserReferences(context.Background(), []string{"@alice", "@bob", "U3"})
	if err != nil {
		t.Fatalf("ResolveUserReferences: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one users.list call, got %d", calls)
	}
	if got := strings.Join(resolved, ","); got != "U1,U2,U3" {
		t.Fatalf("resolved = %s", got)
	}
}
