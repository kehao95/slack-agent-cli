// Package search provides a unified resource abstraction over Slack search APIs.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type Client interface {
	SearchResources(context.Context, string, string, appslack.SearchParams) (*appslack.UnifiedSearchResult, error)
}

type Params struct {
	Kind       string
	Query      string
	Count      int
	Page       int
	All        bool
	SortBy     string
	SortDir    string
	Highlight  bool
	MaxRetries int
	PageDelay  time.Duration
}

type Result struct {
	Query      string                 `json:"query"`
	Kind       string                 `json:"kind"`
	Messages   *appslack.SearchResult `json:"-"`
	Files      *FileMatches           `json:"files,omitempty"`
	Page       int                    `json:"page"`
	PageCount  int                    `json:"page_count"`
	TotalCount int                    `json:"total_count"`
}

// MarshalJSON flattens the legacy SearchResult envelope so the unified shape is
// {query,kind,messages:{total,matches},files:{total,matches},...}.
func (r Result) MarshalJSON() ([]byte, error) {
	type wireResult struct {
		Query      string          `json:"query"`
		Kind       string          `json:"kind"`
		Messages   json.RawMessage `json:"messages,omitempty"`
		Files      *FileMatches    `json:"files,omitempty"`
		Page       int             `json:"page"`
		PageCount  int             `json:"page_count"`
		TotalCount int             `json:"total_count"`
	}
	wire := wireResult{Query: r.Query, Kind: r.Kind, Files: r.Files, Page: r.Page, PageCount: r.PageCount, TotalCount: r.TotalCount}
	if r.Messages != nil {
		encoded, err := json.Marshal(r.Messages)
		if err != nil {
			return nil, err
		}
		var legacy struct {
			Messages json.RawMessage `json:"messages"`
		}
		if err := json.Unmarshal(encoded, &legacy); err != nil {
			return nil, err
		}
		wire.Messages = legacy.Messages
	}
	return json.Marshal(wire)
}

type FileMatches struct {
	Total   int             `json:"total"`
	Matches []slackapi.File `json:"matches"`
}

type Service struct{ client Client }

func NewService(client Client) *Service { return &Service{client: client} }

func (s *Service) Search(ctx context.Context, params Params) (*Result, error) {
	if params.Count <= 0 {
		params.Count = 20
	}
	if params.Count > 100 {
		return nil, fmt.Errorf("search count must not exceed 100")
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	result := &Result{Query: params.Query, Kind: params.Kind, Page: params.Page}
	page := params.Page
	for {
		current, err := appslack.RetryRateLimited(ctx, params.MaxRetries, func() (*appslack.UnifiedSearchResult, error) {
			return s.client.SearchResources(ctx, params.Kind, params.Query, appslack.SearchParams{Count: params.Count, Page: page, SortBy: params.SortBy, SortDir: params.SortDir, Highlight: params.Highlight})
		})
		if err != nil {
			return nil, err
		}
		result.PageCount = current.PageCount
		result.TotalCount = current.TotalCount
		if params.Kind != appslack.SearchKindFiles {
			if result.Messages == nil {
				result.Messages = &appslack.SearchResult{Query: params.Query}
			}
			result.Messages.Messages.Total = current.Messages.Total
			result.Messages.Messages.Matches = append(result.Messages.Messages.Matches, current.Messages.Matches...)
		}
		if params.Kind != appslack.SearchKindMessages {
			if result.Files == nil {
				result.Files = &FileMatches{}
			}
			result.Files.Total = current.Files.Total
			result.Files.Matches = append(result.Files.Matches, current.Files.Matches...)
		}
		if !params.All || current.PageCount == 0 || page >= current.PageCount {
			break
		}
		if err := appslack.WaitContext(ctx, params.PageDelay); err != nil {
			return nil, err
		}
		page++
	}
	return result, nil
}

func (r *Result) Lines() []string {
	lines := []string{fmt.Sprintf("Search %s for %q (%d total)", r.Kind, r.Query, r.TotalCount)}
	if r.Messages != nil {
		for _, match := range r.Messages.Messages.Matches {
			lines = append(lines, fmt.Sprintf("message  %s  %s", match.Timestamp, match.Text))
		}
	}
	if r.Files != nil {
		for _, file := range r.Files.Matches {
			lines = append(lines, fmt.Sprintf("file     %s  %s", file.ID, file.Title))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "No matches found.")
	}
	return lines
}
