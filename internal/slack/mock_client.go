package slack

import (
	"context"

	slackapi "github.com/slack-go/slack"
)

// MockClient implements Client for tests.
type MockClient struct {
	HistoryFunc func(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error)
	ThreadFunc  func(ctx context.Context, params ThreadParams) ([]slackapi.Message, bool, string, error)
}

func (m *MockClient) ListConversationsHistory(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error) {
	if m.HistoryFunc != nil {
		return m.HistoryFunc(ctx, params)
	}
	return nil, slackapi.ErrParametersMissing
}

func (m *MockClient) ListThreadReplies(ctx context.Context, params ThreadParams) ([]slackapi.Message, bool, string, error) {
	if m.ThreadFunc != nil {
		return m.ThreadFunc(ctx, params)
	}
	return nil, false, "", slackapi.ErrParametersMissing
}
