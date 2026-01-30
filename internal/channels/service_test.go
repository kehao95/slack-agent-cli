package channels

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/slack"
)

type mockChannelClient struct {
	listChannels func(ctx context.Context, params slack.ListChannelsParams) ([]slackapi.Channel, string, error)
}

func (m mockChannelClient) ListConversationsHistory(ctx context.Context, params slack.HistoryParams) (*slackapi.GetConversationHistoryResponse, error) {
	return nil, errors.New("not implemented")
}

func (m mockChannelClient) ListThreadReplies(ctx context.Context, params slack.ThreadParams) ([]slackapi.Message, bool, string, error) {
	return nil, false, "", errors.New("not implemented")
}

func (m mockChannelClient) ListChannels(ctx context.Context, params slack.ListChannelsParams) ([]slackapi.Channel, string, error) {
	return m.listChannels(ctx, params)
}

func (m mockChannelClient) JoinChannel(ctx context.Context, channelID string) (*slack.ChannelJoinResult, error) {
	return nil, errors.New("not implemented")
}

func (m mockChannelClient) LeaveChannel(ctx context.Context, channelID string) (*slack.ChannelLeaveResult, error) {
	return nil, errors.New("not implemented")
}

func TestServiceListDefaults(t *testing.T) {
	called := false
	client := mockChannelClient{
		listChannels: func(ctx context.Context, params slack.ListChannelsParams) ([]slackapi.Channel, string, error) {
			called = true
			if params.Limit != 200 {
				t.Fatalf("expected limit 200, got %d", params.Limit)
			}
			if len(params.Types) != len(defaultChannelTypes) {
				t.Fatalf("expected default channel types, got %v", params.Types)
			}
			return []slackapi.Channel{{GroupConversation: slackapi.GroupConversation{Name: "general", Conversation: slackapi.Conversation{ID: "C1"}}}}, "cursor", nil
		},
	}
	service := NewService(client)
	result, err := service.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected ListChannels to be called")
	}
	if len(result.Channels) != 1 || result.NextCursor != "cursor" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestServiceListError(t *testing.T) {
	client := mockChannelClient{
		listChannels: func(ctx context.Context, params slack.ListChannelsParams) ([]slackapi.Channel, string, error) {
			return nil, "", errors.New("boom")
		},
	}
	service := NewService(client)
	_, err := service.List(context.Background(), ListParams{Limit: 100})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestListResultLines(t *testing.T) {
	result := ListResult{Channels: []slackapi.Channel{
		{GroupConversation: slackapi.GroupConversation{Name: "general", Conversation: slackapi.Conversation{ID: "C1"}}},
		{GroupConversation: slackapi.GroupConversation{Name: "private", Conversation: slackapi.Conversation{ID: "C2"}}},
	}}
	lines := result.Lines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
}
