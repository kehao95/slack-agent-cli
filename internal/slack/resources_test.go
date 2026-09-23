package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestListUsersUsesProvidedCursorAndReturnsNextCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("cursor"); got != "incoming" {
			t.Fatalf("cursor=%q", got)
		}
		if got := r.Form.Get("limit"); got != "25" {
			t.Fatalf("limit=%q", got)
		}
		writeJSON(t, w, map[string]any{"ok": true, "members": []map[string]any{{"id": "U1", "name": "alice"}}, "response_metadata": map[string]any{"next_cursor": "outgoing"}})
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	users, cursor, err := client.ListUsers(context.Background(), "incoming", 25)
	if err != nil || len(users) != 1 || cursor != "outgoing" {
		t.Fatalf("users=%#v cursor=%q err=%v", users, cursor, err)
	}
}

func TestSearchResourcesAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.all" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("query") != "incident" || r.Form.Get("page") != "2" {
			t.Fatalf("unexpected form: %v", r.Form)
		}
		writeJSON(t, w, map[string]any{"ok": true, "query": "incident", "messages": map[string]any{"total": 1, "matches": []map[string]any{{"type": "message", "text": "found"}}, "paging": map[string]any{"page": 2, "pages": 3}}, "files": map[string]any{"total": 1, "matches": []map[string]any{{"id": "F1"}}, "paging": map[string]any{"page": 2, "pages": 3}}})
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	result, err := client.SearchResources(context.Background(), SearchKindAll, "incident", SearchParams{Count: 20, Page: 2, SortBy: "score", SortDir: "desc"})
	if err != nil || result.PageCount != 3 || len(result.Messages.Matches) != 1 || len(result.Files.Matches) != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
