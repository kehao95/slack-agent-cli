package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestOpenConversation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.open" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("users"); got != "U1,U2" {
			t.Fatalf("users = %q", got)
		}
		if got := r.Form.Get("return_im"); got != "true" {
			t.Fatalf("return_im = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"G123"},"already_open":true}`))
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	result, err := client.OpenConversation(context.Background(), "", []string{"U1", "U2"})
	if err != nil {
		t.Fatalf("OpenConversation: %v", err)
	}
	if result.Channel.ID != "G123" || !result.AlreadyOpen {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestConversationInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.info" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C123","name":"general"}}`))
	}))
	defer server.Close()
	client := New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
	result, err := client.ConversationInfo(context.Background(), "C123", false, true)
	if err != nil {
		t.Fatalf("ConversationInfo: %v", err)
	}
	if result.Channel.ID != "C123" || result.Channel.Name != "general" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
