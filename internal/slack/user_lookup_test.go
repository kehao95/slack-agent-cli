package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestParseUserReference(t *testing.T) {
	for _, tt := range []struct{ input, id, username string }{
		{"U123", "U123", ""}, {"WABC123", "WABC123", ""},
		{"<@U123>", "U123", ""}, {" <@W123> ", "W123", ""},
		{"@will", "", "will"}, {"@WILL", "", "WILL"}, {"@U123", "", "U123"},
		{"@alice.smith_test-2", "", "alice.smith_test-2"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			id, username, err := ParseUserReference(tt.input)
			if err != nil || id != tt.id || username != tt.username {
				t.Fatalf("got %q, %q, %v", id, username, err)
			}
		})
	}
}

func TestResolveUserReferenceRejectsInvalidBeforeNetwork(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Errorf("unexpected request %s", r.URL.Path) }))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	for _, input := range []string{"", " ", "@", "@@alice", "@ alice", "@alice smith", "alice", "will", "ursula", "u123", "Alice Example", "alice@example.com", "@alice@example.com", "<@U123", "U123>", "<U123>", "<@u123>", "<@alice>", "<@U123|alice>", "<@U123>>", "@<@U123>", "<@ U123>"} {
		t.Run(input, func(t *testing.T) {
			if _, err := client.ResolveUserReference(context.Background(), input); err == nil || !strings.Contains(err.Error(), "invalid user reference") {
				t.Fatalf("expected invalid-reference error, got %v", err)
			}
		})
	}
	if _, err := client.ResolveUserReferences(context.Background(), []string{"@alice", "U123>"}); err == nil {
		t.Fatal("invalid batch must fail before resolving first username")
	}
	if calls != 0 {
		t.Fatalf("got %d network calls", calls)
	}
}

func TestResolveUserReferencesIDsNeedNoClient(t *testing.T) {
	var client APIClient
	got, err := client.ResolveUserReferences(context.Background(), []string{"U123", "<@W456>", "U123"})
	if err != nil || !reflect.DeepEqual(got, []string{"U123", "W456", "U123"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolveUserReferenceUsernameOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"members":[{"id":"U1","name":"alice","real_name":"Alice Example","profile":{"display_name":"Ali","email":"alice@example.com"}},{"id":"U2","name":"bob","profile":{"display_name":"alice"}},{"id":"U3","name":"will"},{"id":"U4","name":"ursula"},{"id":"U5","name":"u123"},{"id":"U6","name":"inactive","deleted":true}]}`)
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	for input, want := range map[string]string{"@alice": "U1", "@ALICE": "U1", "@will": "U3", "@ursula": "U4", "@U123": "U5"} {
		t.Run(input, func(t *testing.T) {
			got, err := client.ResolveUserReference(context.Background(), input)
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
	for _, input := range []string{"@Ali", "@inactive", "@missing"} {
		t.Run(input, func(t *testing.T) {
			if _, err := client.ResolveUserReference(context.Background(), input); err == nil || !strings.Contains(err.Error(), "user not found") {
				t.Fatalf("expected user not found, got %v", err)
			}
		})
	}
}

func TestResolveUserReferenceAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"members":[{"id":"U1","name":"same"},{"id":"U2","name":"same"}]}`)
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	if _, err := client.ResolveUserReference(context.Background(), "@same"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestResolveUserReferencesOnePaginatedTraversal(t *testing.T) {
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		cursor := r.Form.Get("cursor")
		cursors = append(cursors, cursor)
		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "":
			fmt.Fprint(w, `{"ok":true,"members":[{"id":"U1","name":"alice"}],"response_metadata":{"next_cursor":"page2"}}`)
		case "page2":
			fmt.Fprint(w, `{"ok":true,"members":[{"id":"U2","name":"bob"}],"response_metadata":{"next_cursor":""}}`)
		default:
			t.Errorf("unexpected cursor %q", cursor)
		}
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	got, err := client.ResolveUserReferences(context.Background(), []string{"@bob", "U3", "@alice", "<@W4>", "@bob"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"U2", "U3", "U1", "W4", "U2"}) {
		t.Fatalf("got %v", got)
	}
	if !reflect.DeepEqual(cursors, []string{"", "page2"}) {
		t.Fatalf("expected one traversal, got %v", cursors)
	}
}
