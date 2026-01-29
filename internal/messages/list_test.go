package messages

import (
	"context"
	"errors"
	"testing"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/slack"
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
