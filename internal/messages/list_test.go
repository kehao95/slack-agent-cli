package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	var received slack.ThreadParams
	fetcher := mockFetcher{
		listMessages: func(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("unexpected messages call")
		},
		listThread: func(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			received = params
			return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "1", Text: "thread", User: "U1"}}}, "next", false, nil
		},
	}
	service := NewService(fetcher)
	result, err := service.List(context.Background(), Params{Channel: "C", Thread: "1", Cursor: "thread-cursor"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.NextCursor != "next" || result.HasMore {
		t.Fatalf("unexpected result: %+v", result)
	}
	if received.Cursor != "thread-cursor" {
		t.Fatalf("expected thread cursor to be forwarded, got %q", received.Cursor)
	}
}

func TestServiceListAllPages(t *testing.T) {
	calls := 0
	fetcher := mockFetcher{
		listMessages: func(ctx context.Context, params slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			calls++
			switch calls {
			case 1:
				if params.Cursor != "start" {
					t.Fatalf("expected initial cursor start, got %q", params.Cursor)
				}
				return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "1"}}}, "next", true, nil
			case 2:
				if params.Cursor != "next" {
					t.Fatalf("expected next cursor, got %q", params.Cursor)
				}
				return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "2"}}}, "", false, nil
			default:
				return nil, "", false, errors.New("unexpected extra call")
			}
		},
		listThread: func(context.Context, slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("unexpected thread call")
		},
	}
	result, err := NewService(fetcher).List(context.Background(), Params{Channel: "C1", Cursor: "start", All: true})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if calls != 2 || len(result.Messages) != 2 || result.HasMore || result.NextCursor != "" {
		t.Fatalf("unexpected all-pages result: calls=%d result=%+v", calls, result)
	}
}

func TestServiceListRetriesRateLimit(t *testing.T) {
	calls := 0
	fetcher := mockFetcher{
		listMessages: func(context.Context, slack.HistoryParams) ([]slackapi.Message, string, bool, error) {
			calls++
			if calls == 1 {
				return nil, "", false, &slackapi.RateLimitedError{}
			}
			return []slackapi.Message{{Msg: slackapi.Msg{Timestamp: "1"}}}, "", false, nil
		},
		listThread: func(context.Context, slack.ThreadParams) ([]slackapi.Message, string, bool, error) {
			return nil, "", false, errors.New("unexpected thread call")
		},
	}
	result, err := NewService(fetcher).List(context.Background(), Params{
		Channel: "C1", RetryRateLimits: true, MaxRetries: 1,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if calls != 2 || len(result.Messages) != 1 {
		t.Fatalf("expected one retry, calls=%d result=%+v", calls, result)
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

type exactOutputResolver struct {
	handle string
	calls  int
	ids    []string
}

func (r *exactOutputResolver) GetDisplayName(_ context.Context, id string) string {
	r.calls++
	r.ids = append(r.ids, id)
	return "Alice Display"
}

func (r *exactOutputResolver) GetMentionName(_ context.Context, id string) string {
	r.calls++
	r.ids = append(r.ids, id)
	if r.handle == "" {
		return id
	}
	return r.handle
}

func TestMessageIdentityAndRawIsolation(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, handle := range []string{"alice.handle", ""} {
			t.Run(fmt.Sprintf("raw=%t/handle=%s", raw, handle), func(t *testing.T) {
				resolver := &exactOutputResolver{handle: handle}
				original := "Hi <@U123> and <!subteam^S123>"
				result := Result{Channel: "C123", Messages: []slackapi.Message{{Msg: slackapi.Msg{User: "U123", Username: "Bot Alias", Text: original}}}}
				result.SetUserResolver(context.Background(), resolver)
				result.SetRawJSON(raw)
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var output struct {
					Messages []map[string]interface{} `json:"messages"`
				}
				if err := json.Unmarshal(data, &output); err != nil {
					t.Fatal(err)
				}
				msg := output.Messages[0]
				if msg["user"] != "U123" || msg["text"] != original {
					t.Fatalf("identity or source text changed: %s", data)
				}
				if raw {
					if resolver.calls != 0 || msg["username"] != "Bot Alias" || msg["user_id"] != nil || msg["display_name"] != nil {
						t.Fatalf("raw output enriched: calls=%d data=%s", resolver.calls, data)
					}
					return
				}
				if msg["display_name"] != "Alice Display" || msg["user_id"] != "U123" {
					t.Fatalf("missing identity/presentation fields: %s", data)
				}
				if handle == "" && msg["username"] != nil || handle != "" && msg["username"] != "@"+handle {
					t.Fatalf("incorrect handle: %s", data)
				}
				human := strings.Join(result.Lines(), "\n")
				if strings.Contains(human, "@Alice Display") || strings.Contains(human, "@Bot Alias") || strings.Contains(human, "@U123:") {
					t.Fatalf("fabricated human handle: %s", human)
				}
				if handle != "" && !strings.Contains(human, "Hi @alice.handle") {
					t.Fatalf("mention did not use exact handle: %s", human)
				}
			})
		}
	}
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
	if msg1["user"] != "U1" {
		t.Errorf("expected user U1, got %v", msg1["user"])
	}
	if msg1["user_id"] != "U1" {
		t.Errorf("expected user_id U1, got %v", msg1["user_id"])
	}
	if msg1["username"] != "@alice" {
		t.Errorf("expected username @alice, got %v", msg1["username"])
	}
	if msg1["parent_user"] != "U2" {
		t.Errorf("expected parent_user U2, got %v", msg1["parent_user"])
	}
	edited, ok := msg1["edited"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected edited payload, got %T", msg1["edited"])
	}
	if edited["user"] != "U2" {
		t.Errorf("expected edited.user U2, got %v", edited["user"])
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
	if users[0] != "U1" || users[1] != "U2" || users[2] != "U999" {
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
	if file["user"] != "U2" || file["user_id"] != "U2" {
		t.Errorf("unexpected file user fields: %v", file)
	}
	initialComment, ok := file["initial_comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected initial_comment payload, got %T", file["initial_comment"])
	}
	if initialComment["user"] != "U1" || initialComment["user_id"] != "U1" {
		t.Errorf("unexpected initial comment user fields: %v", initialComment)
	}
	replies, ok := msg1["replies"].([]interface{})
	if !ok || len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %v", msg1["replies"])
	}
	reply := replies[0].(map[string]interface{})
	if reply["user"] != "U2" || reply["user_id"] != "U2" {
		t.Errorf("unexpected reply user fields: %v", reply)
	}

	// Second message uses the account handle, not the bot alias
	msg2 := messages[1].(map[string]interface{})
	if msg2["username"] != "@bob" {
		t.Errorf("expected username @bob, got %v", msg2["username"])
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

func TestNestedIdentityReferencesStayCanonical(t *testing.T) {
	result := Result{}
	value := map[string]interface{}{
		"user": "U123", "username": "Bot Alias", "inviter": "U456", "member": "U789",
		"users": []interface{}{"U123", "U456"}, "members": []interface{}{"U789"},
		"previous_message": map[string]interface{}{"user": "U456", "username": "Webhook Alias"},
	}
	result.enrichNestedUserReferences(value)
	for _, field := range []string{"user", "inviter", "member"} {
		if value[field] != value[field+"_id"] {
			t.Fatalf("%s reference changed: %v", field, value)
		}
	}
	if value["user"] != "U123" || value["inviter"] != "U456" || value["member"] != "U789" {
		t.Fatalf("canonical scalar IDs replaced: %v", value)
	}
	if value["users"].([]interface{})[0] != "U123" || value["members"].([]interface{})[0] != "U789" {
		t.Fatalf("canonical array IDs replaced: %v", value)
	}
	if value["user_ids"].([]interface{})[1] != "U456" || value["member_ids"].([]interface{})[0] != "U789" {
		t.Fatalf("compatibility array IDs missing: %v", value)
	}
	nested := value["previous_message"].(map[string]interface{})
	if nested["user"] != "U456" || nested["user_id"] != "U456" || nested["username"] != nil || nested["display_name"] != "Webhook Alias" {
		t.Fatalf("nested alias promoted to identity: %v", nested)
	}
	if value["username"] != nil || value["display_name"] != "Bot Alias" {
		t.Fatalf("alias promoted to identity: %v", value)
	}
}

func TestMessageMetadataRemainsOpaque(t *testing.T) {
	payload := map[string]interface{}{
		"username": "deploy-service", "user": "external-person",
		"users":     []interface{}{"U999", "external-person"},
		"message":   map[string]interface{}{"user": "U999", "username": "Internal Account", "members": []interface{}{"U888"}},
		"arbitrary": []interface{}{map[string]interface{}{"inviter": "U777", "user": "U666"}},
	}
	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprintf("raw=%t", raw), func(t *testing.T) {
			resolver := &exactOutputResolver{handle: "alice.handle"}
			msg := slackapi.Message{Msg: slackapi.Msg{
				User: "U123", Text: "<@U123>",
				Metadata: slackapi.SlackMetadata{EventType: "deployment", EventPayload: payload},
			}}
			result := Result{Channel: "C123", Messages: []slackapi.Message{msg}}
			result.SetUserResolver(context.Background(), resolver)
			result.SetRawJSON(raw)
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var output struct {
				Messages []map[string]interface{} `json:"messages"`
			}
			if err := json.Unmarshal(encoded, &output); err != nil {
				t.Fatal(err)
			}
			metadata := output.Messages[0]["metadata"].(map[string]interface{})
			if metadata["event_type"] != "deployment" || !reflect.DeepEqual(metadata["event_payload"], payload) {
				t.Fatalf("opaque metadata changed: %s", encoded)
			}
			for _, id := range resolver.ids {
				if id != "U123" {
					t.Fatalf("resolved opaque metadata identity %q", id)
				}
			}
			if raw && resolver.calls != 0 {
				t.Fatalf("raw message resolved identities: %v", resolver.ids)
			}
			if !raw && output.Messages[0]["username"] != "@alice.handle" {
				t.Fatalf("message author enrichment missing: %s", encoded)
			}
		})
	}
}

func TestCanonicalOutputUserIDsUseSharedParser(t *testing.T) {
	for _, value := range []string{"U", "W", "U!", "W display", "@alice", "<@U123>", " U123 ", "u123", ""} {
		resolver := &exactOutputResolver{handle: "alice"}
		result := Result{}
		result.SetUserResolver(context.Background(), resolver)
		valueMap := map[string]interface{}{"user": value, "username": "Alias"}
		result.enrichNestedUserReferences(valueMap)
		if resolver.calls != 0 || valueMap["user_id"] != nil || valueMap["username"] != nil || valueMap["user"] != value {
			t.Errorf("noncanonical identity %q enriched: %v calls=%d", value, valueMap, resolver.calls)
		}
	}
}
