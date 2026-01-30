package slack

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"
)

// UserAPIClient wraps operations requiring user token.
type UserAPIClient struct {
	sdk *slackapi.Client
}

// NewUserClient creates a new UserAPIClient using the provided user token.
func NewUserClient(userToken string) *UserAPIClient {
	return &UserAPIClient{sdk: slackapi.New(userToken)}
}

// SearchMessages searches messages across the workspace using search.messages API.
func (c *UserAPIClient) SearchMessages(ctx context.Context, query string, params SearchParams) (*SearchResult, error) {
	if query == "" {
		return nil, ErrQueryRequired
	}

	searchParams := slackapi.SearchParameters{
		Sort:          params.SortBy,
		SortDirection: params.SortDir,
		Count:         params.Count,
		Page:          params.Page,
		Highlight:     params.Highlight,
	}

	messages, err := c.sdk.SearchMessagesContext(ctx, query, searchParams)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}

	// Map slack-go response to our internal structure
	result := &SearchResult{
		Query: query,
		Messages: SearchMessages{
			Total:   messages.Total,
			Matches: make([]SearchMatch, len(messages.Matches)),
		},
	}

	for i, match := range messages.Matches {
		result.Messages.Matches[i] = SearchMatch{
			Type: match.Type,
			Channel: SearchChannel{
				ID:   match.Channel.ID,
				Name: match.Channel.Name,
			},
			User:      match.User,
			Username:  match.Username,
			Timestamp: match.Timestamp,
			Text:      match.Text,
			Permalink: match.Permalink,
		}
	}

	return result, nil
}
