package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListSlackListItems(t *testing.T) {
	var gotAuth string
	var gotCookie string
	var gotPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slackLists.items.list" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeJSON(t, w, map[string]interface{}{
			"ok": true,
			"items": []map[string]interface{}{
				{
					"id": "Ri123",
					"fields": []map[string]interface{}{
						{"text": "Helpdesk request A"},
					},
				},
			},
			"response_metadata": map[string]interface{}{
				"next_cursor": "next",
			},
		})
	}))
	defer server.Close()

	client := &APIClient{
		token:         "xoxp-test-token",
		cookie:        "xoxd-cookie",
		endpoint:      server.URL + "/",
		rawHTTPClient: server.Client(),
	}

	resp, err := client.ListSlackListItems(context.Background(), ListItemsParams{
		ListID:   "F0BFMJY6ZTQ",
		Limit:    3,
		Cursor:   "cursor",
		Archived: true,
	})
	if err != nil {
		t.Fatalf("ListSlackListItems returned error: %v", err)
	}
	if gotAuth != "Bearer xoxp-test-token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotCookie != "d=xoxd-cookie" {
		t.Fatalf("unexpected cookie header: %q", gotCookie)
	}
	if gotPayload["list_id"] != "F0BFMJY6ZTQ" {
		t.Fatalf("unexpected payload: %+v", gotPayload)
	}
	if gotPayload["cursor"] != "cursor" {
		t.Fatalf("unexpected cursor payload: %+v", gotPayload)
	}
	if gotPayload["archived"] != true {
		t.Fatalf("unexpected archived payload: %+v", gotPayload)
	}
	if len(resp.Items) != 1 || resp.ResponseMetadata.NextCursor != "next" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestListSlackListItemsMissingScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{
			"ok":       false,
			"error":    "missing_scope",
			"needed":   "lists:read",
			"provided": "channels:history",
		})
	}))
	defer server.Close()

	client := &APIClient{
		token:         "xoxp-test-token",
		endpoint:      server.URL + "/",
		rawHTTPClient: server.Client(),
	}

	_, err := client.ListSlackListItems(context.Background(), ListItemsParams{ListID: "F0BFMJY6ZTQ"})
	if err == nil {
		t.Fatal("expected missing scope error")
	}
	if !strings.Contains(err.Error(), "missing_scope") || !strings.Contains(err.Error(), "lists:read") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSlackListItemsRequiresListID(t *testing.T) {
	client := &APIClient{}
	_, err := client.ListSlackListItems(context.Background(), ListItemsParams{})
	if err == nil || !strings.Contains(err.Error(), "list is required") {
		t.Fatalf("expected list required error, got %v", err)
	}
}

func TestGetSlackListItem(t *testing.T) {
	var gotPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slackLists.items.info" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeJSON(t, w, map[string]interface{}{
			"ok": true,
			"list": map[string]interface{}{
				"id":    "F0BFMJY6ZTQ",
				"title": "Helpdesk",
			},
			"record": map[string]interface{}{
				"id": "Rec123",
				"fields": []map[string]interface{}{
					{"text": "Helpdesk request A"},
				},
				"is_subscribed": true,
			},
			"subtasks": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	client := &APIClient{
		token:         "xoxp-test-token",
		endpoint:      server.URL + "/",
		rawHTTPClient: server.Client(),
	}

	resp, err := client.GetSlackListItem(context.Background(), ListItemInfoParams{
		ListID:              "F0BFMJY6ZTQ",
		ID:                  "Rec123",
		IncludeIsSubscribed: true,
	})
	if err != nil {
		t.Fatalf("GetSlackListItem returned error: %v", err)
	}
	if gotPayload["list_id"] != "F0BFMJY6ZTQ" || gotPayload["id"] != "Rec123" || gotPayload["include_is_subscribed"] != true {
		t.Fatalf("unexpected payload: %+v", gotPayload)
	}
	if resp.Record["id"] != "Rec123" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetSlackListItemRequiresRecordID(t *testing.T) {
	client := &APIClient{}
	_, err := client.GetSlackListItem(context.Background(), ListItemInfoParams{ListID: "F0BFMJY6ZTQ"})
	if err == nil || !strings.Contains(err.Error(), "record id is required") {
		t.Fatalf("expected record id required error, got %v", err)
	}
}

func TestGetSlackList(t *testing.T) {
	var gotForm map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files.info" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = map[string]string{
			"file": r.Form.Get("file"),
		}
		writeJSON(t, w, map[string]interface{}{
			"ok": true,
			"file": map[string]interface{}{
				"id":    "F0BFMJY6ZTQ",
				"title": "VOC Helpdesk tracker",
				"list_metadata": map[string]interface{}{
					"schema": []map[string]interface{}{
						{"id": "Col1", "name": "Request", "key": "name", "type": "text"},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := &APIClient{
		token:         "xoxp-test-token",
		endpoint:      server.URL + "/",
		rawHTTPClient: server.Client(),
	}

	resp, err := client.GetSlackList(context.Background(), "F0BFMJY6ZTQ")
	if err != nil {
		t.Fatalf("GetSlackList returned error: %v", err)
	}
	if gotForm["file"] != "F0BFMJY6ZTQ" {
		t.Fatalf("unexpected form payload: %+v", gotForm)
	}
	if resp.File["title"] != "VOC Helpdesk tracker" {
		t.Fatalf("unexpected response: %+v", resp.File)
	}
}
