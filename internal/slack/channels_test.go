package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestListChannelsUsesConversationsList(t *testing.T) {
	var gotPath string
	var gotExcludeArchived string
	var gotCursor string
	var gotLimit string
	var gotTypes string
	var parseErr error

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		parseErr = r.ParseForm()
		if parseErr == nil {
			gotExcludeArchived = r.Form.Get("exclude_archived")
			gotCursor = r.Form.Get("cursor")
			gotLimit = r.Form.Get("limit")
			gotTypes = r.Form.Get("types")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"channels": [{
				"id": "CARCHIVED",
				"name": "old-public-channel",
				"is_archived": true,
				"is_member": false
			}],
			"response_metadata": {"next_cursor": "next-page"}
		}`))
	}))
	defer server.Close()

	client := New("xoxp-test-token", slackapi.OptionAPIURL(server.URL+"/"))
	channels, nextCursor, err := client.ListChannels(context.Background(), ListChannelsParams{
		Limit:           123,
		Cursor:          "current-page",
		IncludeArchived: true,
		Types:           []string{"public_channel", "private_channel"},
	})
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}

	if parseErr != nil {
		t.Fatalf("parse form: %v", parseErr)
	}
	if gotPath != "/conversations.list" {
		t.Fatalf("expected conversations.list API, got %q", gotPath)
	}
	if gotExcludeArchived != "" {
		t.Fatalf("expected exclude_archived to be omitted, got %q", gotExcludeArchived)
	}
	if gotCursor != "current-page" {
		t.Fatalf("expected cursor current-page, got %q", gotCursor)
	}
	if gotLimit != "123" {
		t.Fatalf("expected limit 123, got %q", gotLimit)
	}
	if gotTypes != "public_channel,private_channel" {
		t.Fatalf("expected requested channel types, got %q", gotTypes)
	}
	if len(channels) != 1 || channels[0].ID != "CARCHIVED" || !channels[0].IsArchived || channels[0].IsMember {
		t.Fatalf("unexpected channels: %+v", channels)
	}
	if nextCursor != "next-page" {
		t.Fatalf("expected next cursor next-page, got %q", nextCursor)
	}
}

func TestListChannelsExcludesArchivedByDefault(t *testing.T) {
	var gotExcludeArchived string
	var parseErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parseErr = r.ParseForm()
		if parseErr == nil {
			gotExcludeArchived = r.Form.Get("exclude_archived")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[],"response_metadata":{"next_cursor":""}}`))
	}))
	defer server.Close()

	client := New("xoxp-test-token", slackapi.OptionAPIURL(server.URL+"/"))
	_, _, err := client.ListChannels(context.Background(), ListChannelsParams{})
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if parseErr != nil {
		t.Fatalf("parse form: %v", parseErr)
	}
	if gotExcludeArchived != "true" {
		t.Fatalf("expected exclude_archived=true, got %q", gotExcludeArchived)
	}
}
