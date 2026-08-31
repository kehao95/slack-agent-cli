package search

import (
	"context"
	"encoding/json"
	"testing"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type mockSearchClient struct {
	pages []appslack.UnifiedSearchResult
	calls int
}

func (m *mockSearchClient) SearchResources(_ context.Context, _, _ string, _ appslack.SearchParams) (*appslack.UnifiedSearchResult, error) {
	result := m.pages[m.calls]
	m.calls++
	return &result, nil
}

func TestSearchAggregatesAllPages(t *testing.T) {
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{
		{Messages: appslack.SearchMessages{Total: 2, Matches: []appslack.SearchMatch{{Text: "one"}}}, Files: appslack.SearchFiles{Total: 2, Matches: []slackapi.File{{ID: "F1"}}}, PageCount: 2, TotalCount: 4},
		{Messages: appslack.SearchMessages{Total: 2, Matches: []appslack.SearchMatch{{Text: "two"}}}, Files: appslack.SearchFiles{Total: 2, Matches: []slackapi.File{{ID: "F2"}}}, PageCount: 2, TotalCount: 4},
	}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindAll, Query: "x", Count: 20, Page: 1, All: true})
	if err != nil {
		t.Fatal(err)
	}
	if mock.calls != 2 || len(result.Messages.Messages.Matches) != 2 || len(result.Files.Matches) != 2 {
		t.Fatalf("unexpected result: %#v calls=%d", result, mock.calls)
	}
}

func TestSearchRejectsOversizedPage(t *testing.T) {
	_, err := NewService(&mockSearchClient{}).Search(context.Background(), Params{Count: 101})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestResultJSONUsesFlatResourceEnvelopes(t *testing.T) {
	result := Result{Query: "x", Kind: appslack.SearchKindAll, Messages: &appslack.SearchResult{Query: "x", Messages: appslack.SearchMessages{Total: 1, Matches: []appslack.SearchMatch{{Text: "found"}}}}, Files: &FileMatches{Total: 1, Matches: []slackapi.File{{ID: "F1"}}}}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	messages, ok := decoded["messages"].(map[string]any)
	if !ok {
		t.Fatalf("messages is not an object: %s", encoded)
	}
	if _, nested := messages["messages"]; nested {
		t.Fatalf("legacy envelope was not flattened: %s", encoded)
	}
	if messages["total"].(float64) != 1 {
		t.Fatalf("unexpected total: %s", encoded)
	}
}
