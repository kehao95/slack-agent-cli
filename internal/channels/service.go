package channels

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/slack"
)

type Service struct {
	client slack.ChannelClient
}

func NewService(client slack.ChannelClient) *Service {
	return &Service{client: client}
}

// Default to public channels only - private_channel requires groups:read scope
var defaultChannelTypes = []string{"public_channel"}

type ListParams struct {
	Limit           int
	Cursor          string
	IncludeArchived bool
	Types           []string
}

type ListResult struct {
	Channels   []slackapi.Channel `json:"channels"`
	NextCursor string             `json:"next_cursor"`
}

func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 200
	}
	types := effectiveTypes(params.Types)
	chans, cursor, err := s.client.ListChannels(ctx, slack.ListChannelsParams{
		Limit:           params.Limit,
		Cursor:          params.Cursor,
		IncludeArchived: params.IncludeArchived,
		Types:           types,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("list channels: %w", err)
	}
	return ListResult{Channels: chans, NextCursor: cursor}, nil
}

func effectiveTypes(types []string) []string {
	if len(types) == 0 {
		return append([]string{}, defaultChannelTypes...)
	}
	return types
}

func (r ListResult) Lines() []string {
	if len(r.Channels) == 0 {
		return []string{"No channels found."}
	}
	title := fmt.Sprintf("Channels (%d)", len(r.Channels))
	lines := []string{title, strings.Repeat("-", len(title))}
	for _, ch := range r.Channels {
		privacy := "public"
		if ch.IsPrivate {
			privacy = "private"
		}
		lines = append(lines, fmt.Sprintf("%s (%s) - %s", ch.Name, ch.ID, privacy))
	}
	if r.NextCursor != "" {
		lines = append(lines, fmt.Sprintf("Next cursor: %s", r.NextCursor))
	}
	return lines
}
