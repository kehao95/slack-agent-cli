package slack

import (
	"context"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	SearchKindAll      = "all"
	SearchKindMessages = "messages"
	SearchKindFiles    = "files"
)

// UnifiedSearchResult is one page returned by search.all, search.messages, or search.files.
type UnifiedSearchResult struct {
	Query      string         `json:"query"`
	Kind       string         `json:"kind"`
	Messages   SearchMessages `json:"messages"`
	Files      SearchFiles    `json:"files"`
	Page       int            `json:"page"`
	PageCount  int            `json:"page_count"`
	TotalCount int            `json:"total_count"`
}

// SearchFiles contains file search matches.
type SearchFiles struct {
	Total   int             `json:"total"`
	Matches []slackapi.File `json:"matches"`
}

// SearchResources searches a single Slack resource family or both families.
func (c *APIClient) SearchResources(ctx context.Context, kind, query string, params SearchParams) (*UnifiedSearchResult, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrQueryRequired
	}
	if kind != SearchKindAll && kind != SearchKindMessages && kind != SearchKindFiles {
		return nil, fmt.Errorf("unsupported search kind %q", kind)
	}
	sdkParams := slackapi.SearchParameters{Sort: params.SortBy, SortDirection: params.SortDir, Count: params.Count, Page: params.Page, Highlight: params.Highlight}
	result := &UnifiedSearchResult{Query: query, Kind: kind, Page: params.Page}
	if result.Page <= 0 {
		result.Page = 1
	}

	var sdkMessages *slackapi.SearchMessages
	var sdkFiles *slackapi.SearchFiles
	var err error
	switch kind {
	case SearchKindAll:
		sdkMessages, sdkFiles, err = c.sdk.SearchContext(ctx, query, sdkParams)
	case SearchKindMessages:
		sdkMessages, err = c.sdk.SearchMessagesContext(ctx, query, sdkParams)
	case SearchKindFiles:
		sdkFiles, err = c.sdk.SearchFilesContext(ctx, query, sdkParams)
	}
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", kind, err)
	}
	if sdkMessages != nil {
		result.Messages.Total = sdkMessages.Total
		result.Messages.Matches = mapSearchMessageMatches(sdkMessages.Matches)
		result.PageCount = maxInt(result.PageCount, searchPageCount(sdkMessages.Paging, sdkMessages.Pagination))
		result.TotalCount += sdkMessages.Total
	}
	if sdkFiles != nil {
		result.Files.Total = sdkFiles.Total
		result.Files.Matches = sdkFiles.Matches
		result.PageCount = maxInt(result.PageCount, searchPageCount(sdkFiles.Paging, sdkFiles.Pagination))
		result.TotalCount += sdkFiles.Total
	}
	return result, nil
}

func mapSearchMessageMatches(matches []slackapi.SearchMessage) []SearchMatch {
	result := make([]SearchMatch, len(matches))
	for i, match := range matches {
		result[i] = SearchMatch{Type: match.Type, Channel: SearchChannel{ID: match.Channel.ID, Name: match.Channel.Name}, User: match.User, Username: match.Username, Timestamp: match.Timestamp, Text: match.Text, Permalink: match.Permalink}
	}
	return result
}

func searchPageCount(paging slackapi.Paging, pagination slackapi.Pagination) int {
	if paging.Pages > 0 {
		return paging.Pages
	}
	return pagination.PageCount
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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

	result.Messages.Matches = mapSearchMessageMatches(messages.Matches)

	return result, nil
}
