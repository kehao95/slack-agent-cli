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
	NextPage   int                    `json:"next_page,omitempty"`
	HasMore    bool                   `json:"has_more"`
}

// MarshalJSON flattens the legacy SearchResult envelope so the unified shape is
// {query,kind,messages:{total,matches},files:{total,matches},...}.
func (r Result) MarshalJSON() ([]byte, error) {
	type wireResult struct {
		Query         string          `json:"query"`
		Kind          string          `json:"kind"`
		Messages      json.RawMessage `json:"messages,omitempty"`
		Files         *FileMatches    `json:"files,omitempty"`
		Page          int             `json:"page"`
		PageCount     int             `json:"page_count"`
		TotalCount    int             `json:"total_count"`
		ReturnedCount int             `json:"returned_count"`
		NextPage      int             `json:"next_page,omitempty"`
		HasMore       bool            `json:"has_more"`
		Incomplete    bool            `json:"incomplete"`
	}
	wire := wireResult{Query: r.Query, Kind: r.Kind, Files: r.Files, Page: r.Page, PageCount: r.PageCount, TotalCount: r.TotalCount, NextPage: r.NextPage, HasMore: r.HasMore}
	if r.Files != nil {
		wire.ReturnedCount += len(r.Files.Matches)
	}
	if r.Messages != nil {
		wire.ReturnedCount += len(r.Messages.Messages.Matches)
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
	wire.Incomplete = r.incomplete()
	return json.Marshal(wire)
}

func (r Result) incomplete() bool {
	return r.Messages != nil && len(r.Messages.Messages.Matches) < r.Messages.Messages.Total ||
		r.Files != nil && len(r.Files.Matches) < r.Files.Total
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
	seenMessages := make(map[[2]string]struct{})
	seenFiles := make(map[string]struct{})
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
			for _, match := range current.Messages.Matches {
				if match.Channel.ID != "" && match.Timestamp != "" {
					key := [2]string{match.Channel.ID, match.Timestamp}
					if _, seen := seenMessages[key]; seen {
						continue
					}
					seenMessages[key] = struct{}{}
				}
				result.Messages.Messages.Matches = append(result.Messages.Messages.Matches, match)
			}
		}
		if params.Kind != appslack.SearchKindMessages {
			if result.Files == nil {
				result.Files = &FileMatches{}
			}
			result.Files.Total = current.Files.Total
			for _, file := range current.Files.Matches {
				if file.ID != "" {
					if _, seen := seenFiles[file.ID]; seen {
						continue
					}
					seenFiles[file.ID] = struct{}{}
				}
				result.Files.Matches = append(result.Files.Matches, file)
			}
		}
		result.HasMore = page < current.PageCount
		result.NextPage = 0
		if result.HasMore {
			result.NextPage = page + 1
		}
		if !params.All || !result.HasMore {
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
	returned := 0
	if r.Messages != nil {
		returned += len(r.Messages.Messages.Matches)
	}
	if r.Files != nil {
		returned += len(r.Files.Matches)
	}
	lines := []string{fmt.Sprintf("Search %s for %q (%d returned of %d total)", r.Kind, r.Query, returned, r.TotalCount)}
	if r.Messages != nil {
		for _, match := range r.Messages.Messages.Matches {
			lines = append(lines, fmt.Sprintf("message  %s  %s", match.Timestamp, match.Channel.Name))
			for _, line := range match.ContentLines() {
				lines = append(lines, "  "+line)
			}
			if match.Permalink != "" {
				lines = append(lines, "  "+match.Permalink)
			}
		}
	}
	if r.Files != nil {
		for _, file := range r.Files.Matches {
			lines = append(lines, fmt.Sprintf("file     %s  %s", file.ID, file.Title))
			if file.Permalink != "" {
				lines = append(lines, "  "+file.Permalink)
			}
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "No matches found.")
	}
	if r.HasMore {
		lines = append(lines, fmt.Sprintf("More results: repeat this search with --page %d, keeping the same --limit and sort options.", r.NextPage))
	} else if r.incomplete() {
		lines = append(lines, "Incomplete results: no further page was provided. Earlier pages may be excluded; narrow the query or read known permalinks.")
	}
	return lines
}
