package messages

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/slack"
)

type mockFetcher struct {
	listMessages func(context.Context, slack.HistoryParams) ([]slackapi.Message, string, bool, error)
	listThread   func(context.Context, slack.ThreadParams) ([]slackapi.Message, string, bool, error)
}

func (m mockFetcher) ListMessages(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
	return m.listMessages(ctx, params)
}

func (m mockFetcher) ListThread(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
	return m.listThread(ctx, params)
}

func TestServiceListChannel(t *testing.T) {
	fetcher := mockFetcher{
		listMessages: func(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "1", Text: "hello", User: "U1"}}}, "cursor", true, nil
		},
		listThread: func(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("unexpected thread call")
		},
	}
	service := NewService(fetcher)
	result, err := service.List(context.Background(), Params{Channel: "C", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Messages) != 1 || result.NextCursor != "cursor" || !result.HasMore {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestServiceListThread(t *testing.T) {
	fetcher := mockFetcher{
		listMessages: func(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("unexpected messages call")
		},
		listThread: func(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "1", Text: "thread", User: "U1"}}}, "next", false, nil
		},
	}
	service := NewService(fetcher)
	result, err := service.List(context.Background(), Params{Channel: "C", Thread: "1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.NextCursor != "next" || result.HasMore {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestServiceListError(t *testing.T) {
	fetcher := mockFetcher{
		listMessages: func(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("boom")
		},
		listThread: func(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, nil
		},
	}
	service := NewService(fetcher)
	_, err := service.List(context.Background(), Params{Channel: "C"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResultLines(t *testing.T) {
	result := Result{
		Channel: "#general",
		Messages: []slackapi.Message{
			{Msg: slackapi.Msg{Timestamp: "1", User: "U1", Text: "Hello"}},
		},
		NextCursor: "abc",
	}
	lines := result.Lines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
}

type mockUserResolver struct {
	users map[string]string
}

func (m mockUserResolver) GetDisplayName(ctx context.Context, userID string) string {
	if name, ok := m.users[userID]; ok {
		return name
	}
	return userID
}

func (m mockUserResolver) GetMentionName(ctx context.Context, userID string) string {
	if name, ok := m.users[userID]; ok {
		return strings.ToLower(strings.ReplaceAll(name, " ", "."))
	}
	return userID
}

func TestResultMarshalJSON_WithUsernames(t *testing.T) {
	resolver := mockUserResolver{
		users: map[string]string{
			"U1": "alice",
			"U2": "bob",
		},
	}

	result := Result{
		Channel:     "C123",
		ChannelName: "general",
		Messages: []slackapi.Message{
			{Msg: slackapi.Msg{Timestamp: "1", User: "U1", Text: "Hello", ParentUserId: "U2", Edited: &slackapi.Edited{User: "U2", Timestamp: "9"}, Reactions: []slackapi.ItemReaction{{Name: "+1", Count: 2, Users: []string{"U1", "U2", "U999"}}}, Replies: []slackapi.Reply{{User: "U2", Timestamp: "10"}}, Files: []slackapi.File{{ID: "F1", User: "U2", InitialComment: slackapi.Comment{User: "U1", Comment: "note"}}}}},
			{Msg: slackapi.Msg{Timestamp: "2", User: "U2", Text: "World", Username: "bot"}},
			{Msg: slackapi.Msg{Timestamp: "3", User: "U999", Text: "Unknown"}},
		},
	}
	result.SetUserResolver(context.Background(), resolver)

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("unmarshal output failed: %v", err)
	}

	// Verify channel fields
	if output["channel"] != "#general" {
		t.Errorf("expected channel #general, got %v", output["channel"])
	}
	if output["channel_id"] != "C123" {
		t.Errorf("expected channel_id C123, got %v", output["channel_id"])
	}
	if output["channel_name"] != "general" {
		t.Errorf("expected channel_name general, got %v", output["channel_name"])
	}

	// Verify messages with usernames
	messages, ok := output["messages"].([]interface{})
	if !ok || len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// First message should have resolved username
	msg1 := messages[0].(map[string]interface{})
	if msg1["user"] != "@alice" {
		t.Errorf("expected user @alice, got %v", msg1["user"])
	}
	if msg1["user_id"] != "U1" {
		t.Errorf("expected user_id U1, got %v", msg1["user_id"])
	}
	if msg1["username"] != "alice" {
		t.Errorf("expected username alice, got %v", msg1["username"])
	}
	if msg1["parent_user"] != "@bob" {
		t.Errorf("expected parent_user @bob, got %v", msg1["parent_user"])
	}
	edited, ok := msg1["edited"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected edited payload, got %T", msg1["edited"])
	}
	if edited["user"] != "@bob" {
		t.Errorf("expected edited.user @bob, got %v", edited["user"])
	}
	if edited["user_id"] != "U2" {
		t.Errorf("expected edited.user_id U2, got %v", edited["user_id"])
	}
	reactions, ok := msg1["reactions"].([]interface{})
	if !ok || len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %v", msg1["reactions"])
	}
	reaction := reactions[0].(map[string]interface{})
	users, ok := reaction["users"].([]interface{})
	if !ok || len(users) != 3 {
		t.Fatalf("expected 3 resolved users, got %v", reaction["users"])
	}
	if users[0] != "@alice" || users[1] != "@bob" || users[2] != "U999" {
		t.Errorf("unexpected resolved reaction users: %v", users)
	}
	userIDs, ok := reaction["user_ids"].([]interface{})
	if !ok || len(userIDs) != 3 {
		t.Fatalf("expected 3 raw user ids, got %v", reaction["user_ids"])
	}
	if userIDs[0] != "U1" || userIDs[1] != "U2" || userIDs[2] != "U999" {
		t.Errorf("unexpected raw reaction user ids: %v", userIDs)
	}
	files, ok := msg1["files"].([]interface{})
	if !ok || len(files) != 1 {
		t.Fatalf("expected 1 file, got %v", msg1["files"])
	}
	file := files[0].(map[string]interface{})
	if file["user"] != "@bob" || file["user_id"] != "U2" {
		t.Errorf("unexpected file user fields: %v", file)
	}
	initialComment, ok := file["initial_comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected initial_comment payload, got %T", file["initial_comment"])
	}
	if initialComment["user"] != "@alice" || initialComment["user_id"] != "U1" {
		t.Errorf("unexpected initial comment user fields: %v", initialComment)
	}
	replies, ok := msg1["replies"].([]interface{})
	if !ok || len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %v", msg1["replies"])
	}
	reply := replies[0].(map[string]interface{})
	if reply["user"] != "@bob" || reply["user_id"] != "U2" {
		t.Errorf("unexpected reply user fields: %v", reply)
	}

	// Second message should use existing username field
	msg2 := messages[1].(map[string]interface{})
	if msg2["username"] != "bot" {
		t.Errorf("expected username bot, got %v", msg2["username"])
	}

	// Third message should have no username (unresolved)
	msg3 := messages[2].(map[string]interface{})
	if _, exists := msg3["username"]; exists {
		t.Errorf("expected no username for unresolved user, got %v", msg3["username"])
	}
}

func TestResultMarshalJSON_RawJSON(t *testing.T) {
	result := Result{
		Channel:     "C123",
		ChannelName: "general",
		Messages: []slackapi.Message{
			{Msg: slackapi.Msg{Timestamp: "1", User: "U1", Text: "Hello"}},
		},
	}
	result.SetRawJSON(true)

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("unmarshal output failed: %v", err)
	}

	if output["channel"] != "C123" {
		t.Fatalf("expected raw channel C123, got %v", output["channel"])
	}
	if _, exists := output["channel_id"]; exists {
		t.Fatalf("did not expect channel_id in raw mode, got %v", output["channel_id"])
	}

	messages := output["messages"].([]interface{})
	msg1 := messages[0].(map[string]interface{})
	if msg1["user"] != "U1" {
		t.Fatalf("expected raw user U1, got %v", msg1["user"])
	}
	if _, exists := msg1["user_id"]; exists {
		t.Fatalf("did not expect user_id in raw mode, got %v", msg1["user_id"])
	}
	if _, exists := msg1["username"]; exists {
		t.Errorf("expected no username without resolver, got %v", msg1["username"])
	}
}
