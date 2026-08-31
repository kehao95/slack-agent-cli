package slack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	slackapi "github.com/slack-go/slack"
)

func TestCallAPIUsesFormAuthAndCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer xoxp-test" {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Cookie") != "d=xoxd-cookie" {
			t.Fatalf("unexpected cookie: %q", r.Header.Get("Cookie"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("channel") != "C123" || r.Form.Get("include_num_members") != "true" || r.Form.Get("users") != `["U1","U2"]` {
			t.Fatalf("unexpected form: %v", r.Form)
		}
		writeJSON(t, w, map[string]interface{}{"ok": true, "channel": map[string]interface{}{"id": "C123"}})
	}))
	defer server.Close()

	client := &APIClient{token: "xoxp-test", cookie: "xoxd-cookie", endpoint: server.URL, rawHTTPClient: server.Client()}
	response, err := client.CallAPI(context.Background(), "conversations.info", map[string]interface{}{"channel": "C123", "include_num_members": true, "users": []string{"U1", "U2"}}, CallAPIOptions{})
	if err != nil {
		t.Fatalf("CallAPI: %v", err)
	}
	if !strings.Contains(string(response), `"C123"`) {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestCallAPIRetriesRateLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	var waited time.Duration
	client := &APIClient{
		token: "xoxp-test", endpoint: server.URL, rawHTTPClient: server.Client(),
		retryWait: func(_ context.Context, delay time.Duration) error {
			waited = delay
			return nil
		},
	}
	if _, err := client.CallAPI(context.Background(), "api.test", nil, CallAPIOptions{MaxRetries: 1}); err != nil {
		t.Fatalf("CallAPI: %v", err)
	}
	if requests != 2 || waited != 7*time.Second {
		t.Fatalf("requests=%d waited=%s", requests, waited)
	}
}

func TestCallAPIReturnsRateLimitAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &APIClient{token: "xoxp-test", endpoint: server.URL, rawHTTPClient: server.Client()}
	_, err := client.CallAPI(context.Background(), "api.test", nil, CallAPIOptions{})
	var rateLimitErr *slackapi.RateLimitedError
	if !errors.As(err, &rateLimitErr) || rateLimitErr.RetryAfter != 3*time.Second {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestCallAPIReturnsSlackErrorDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{
			"ok": false, "error": "missing_scope", "needed": "channels:read", "provided": "users:read",
		})
	}))
	defer server.Close()

	client := &APIClient{token: "xoxp-test", endpoint: server.URL, rawHTTPClient: server.Client()}
	_, err := client.CallAPI(context.Background(), "conversations.list", nil, CallAPIOptions{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "missing_scope" || apiErr.Needed != "channels:read" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallAPIRejectsInvalidMethodAndResponse(t *testing.T) {
	client := &APIClient{rawHTTPClient: http.DefaultClient}
	if _, err := client.CallAPI(context.Background(), "../oauth", nil, CallAPIOptions{}); err == nil {
		t.Fatal("expected invalid method error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()
	client = &APIClient{token: "x", endpoint: server.URL, rawHTTPClient: server.Client()}
	if _, err := client.CallAPI(context.Background(), "api.test", nil, CallAPIOptions{}); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestNextCursor(t *testing.T) {
	response := json.RawMessage(`{"ok":true,"response_metadata":{"next_cursor":" next "}}`)
	if got := NextCursor(response); got != "next" {
		t.Fatalf("NextCursor = %q", got)
	}
	if got := NextCursor(json.RawMessage(`{"ok":true}`)); got != "" {
		t.Fatalf("NextCursor without metadata = %q", got)
	}
}
