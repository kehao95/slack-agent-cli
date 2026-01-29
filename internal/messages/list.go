package messages

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/slack"
)

// Service coordinates message list operations.
type Fetcher interface {
	ListMessages(context.Context, slack.HistoryParams) ([]slackapi.Message, string, bool, error)
	ListThread(context.Context, slack.ThreadParams) ([]slackapi.Message, string, bool, error)
}

// Service coordinates message list operations.
type Service struct {
	fetcher Fetcher
}

// NewService constructs a Service.
func NewService(fetcher Fetcher) *Service {
	return &Service{fetcher: fetcher}
}

// Params describes input for List.
type Params struct {
	Channel string
	Limit   int
	Since   string
	Until   string
	Thread  string
	Cursor  string
}

// Result represents list output.
type Result struct {
	Channel    string             `json:"channel"`
	Messages   []slackapi.Message `json:"messages"`
	HasMore    bool               `json:"has_more"`
	NextCursor string             `json:"next_cursor"`
}

// List retrieves channel or thread history.
func (s *Service) List(ctx context.Context, params Params) (Result, error) {
	if params.Channel == "" {
		return Result{}, fmt.Errorf("channel is required")
	}
	oldest, latest, err := slack.ParseTimeRange(params.Since, params.Until)
	if err != nil {
		return Result{}, err
	}
	if params.Thread != "" {
		msgs, cursor, more, err := s.fetcher.ListThread(ctx, slack.ThreadParams{
			Channel: params.Channel,
			Limit:   params.Limit,
			Latest:  latest,
			Oldest:  oldest,
			Thread:  params.Thread,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{Channel: params.Channel, Messages: msgs, HasMore: more, NextCursor: cursor}, nil
	}
	msgs, cursor, more, err := s.fetcher.ListMessages(ctx, slack.HistoryParams{
		Channel:   params.Channel,
		Limit:     params.Limit,
		Cursor:    params.Cursor,
		Latest:    latest,
		Oldest:    oldest,
		Inclusive: false,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Channel: params.Channel, Messages: msgs, HasMore: more, NextCursor: cursor}, nil
}

// Lines returns human-readable lines for Result.
func (r Result) Lines() []string {
	title := fmt.Sprintf("%s - %d messages", r.Channel, len(r.Messages))
	lines := []string{title, strings.Repeat("-", len(title))}
	for _, msg := range r.Messages {
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", msg.Msg.Timestamp, displayUser(msg), msg.Msg.Text))
	}
	if r.NextCursor != "" {
		lines = append(lines, fmt.Sprintf("Next cursor: %s", r.NextCursor))
	}
	return lines
}

func displayUser(msg slackapi.Message) string {
	if msg.Username != "" {
		return msg.Username
	}
	if msg.Msg.User != "" {
		return msg.Msg.User
	}
	return "unknown"
}
