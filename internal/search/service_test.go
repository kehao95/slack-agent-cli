package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	if result.HasMore || result.NextPage != 0 {
		t.Fatalf("completed pagination reports more pages: %#v", result)
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

func TestUnifiedHumanSearchIncludesStructuredContent(t *testing.T) {
	var match appslack.SearchMatch
	if err := json.Unmarshal([]byte(`{"text":"","ts":"1.0","permalink":"https://example.test/message","attachments":[{"title":"Capacity","text":"No GPU capacity","title_link":"https://example.test/dashboard"}],"blocks":[{"type":"section","text":{"type":"mrkdwn","text":"Runbook details"}}]}`), &match); err != nil {
		t.Fatal(err)
	}
	result := Result{Query: "capacity", Kind: appslack.SearchKindMessages, TotalCount: 1,
		Messages: &appslack.SearchResult{Messages: appslack.SearchMessages{Total: 1, Matches: []appslack.SearchMatch{match}}}}
	human := strings.Join(result.Lines(), "\n")
	for _, want := range []string{"No GPU capacity", "Runbook details", "https://example.test/message", "https://example.test/dashboard"} {
		if !strings.Contains(human, want) {
			t.Errorf("human output omitted %q: %s", want, human)
		}
	}
}

func TestUnifiedSearchReportsContinuation(t *testing.T) {
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{
		{Messages: appslack.SearchMessages{Total: 3, Matches: []appslack.SearchMatch{{Text: "one"}}}, PageCount: 3, TotalCount: 3},
	}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindMessages, Query: "x", Count: 1, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["has_more"] != true || fields["next_page"] != float64(2) || fields["returned_count"] != float64(1) {
		t.Errorf("partial page was not disclosed: %s", encoded)
	}
	if !strings.Contains(strings.Join(result.Lines(), "\n"), "--page 2") {
		t.Errorf("human output hides continuation: %v", result.Lines())
	}
}

func TestEmptySearchReportsNoContinuation(t *testing.T) {
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{{}}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindMessages, Query: "empty", All: true})
	if err != nil {
		t.Fatal(err)
	}
	if mock.calls != 1 || result.HasMore || result.NextPage != 0 {
		t.Fatalf("empty search should stop: calls=%d result=%#v", mock.calls, result)
	}
	if !strings.Contains(strings.Join(result.Lines(), "\n"), "No matches found.") {
		t.Fatalf("empty results not disclosed: %v", result.Lines())
	}
}

func TestSearchAllDoesNotRepeatExhaustedResourceFamily(t *testing.T) {
	for _, exhausted := range []string{appslack.SearchKindMessages, appslack.SearchKindFiles} {
		t.Run(exhausted, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				page := r.FormValue("page")
				if page == "" {
					page = "1"
				}
				current, err := strconv.Atoi(page)
				if err != nil {
					t.Error(err)
				}
				messagePage, filePage := current, current
				messagePages, filePages := 2, 2
				if exhausted == appslack.SearchKindMessages {
					messagePage, messagePages = 1, 1
				} else {
					filePage, filePages = 1, 1
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":       true,
					"messages": map[string]any{"total": messagePages, "paging": map[string]any{"page": messagePage, "pages": messagePages}, "matches": []map[string]any{{"ts": strconv.Itoa(messagePage), "text": "message"}}},
					"files":    map[string]any{"total": filePages, "pagination": map[string]any{"page": filePage, "page_count": filePages}, "matches": []map[string]any{{"id": "F" + strconv.Itoa(filePage)}}},
				})
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := NewService(client).Search(context.Background(), Params{Kind: appslack.SearchKindAll, Query: "synthetic", Count: 1, All: true})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || len(result.Messages.Messages.Matches)+len(result.Files.Matches) != 3 {
				t.Fatalf("repeated exhausted resource family: messages=%d files=%d calls=%d", len(result.Messages.Messages.Matches), len(result.Files.Matches), calls)
			}
		})
	}
}

func TestSearchMissingPaginationDisclosesIncompleteResults(t *testing.T) {
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{
		{Messages: appslack.SearchMessages{Total: 10, Matches: []appslack.SearchMatch{{Text: "only returned match"}}}, TotalCount: 10},
	}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindMessages, Query: "x", All: true})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["incomplete"] != true {
		t.Errorf("missing pagination silently implies complete results: %s", encoded)
	}
	if mock.calls != 1 || !strings.Contains(strings.Join(result.Lines(), "\n"), "no further page") {
		t.Errorf("missing pagination must stop with a visible omission: calls=%d lines=%v", mock.calls, result.Lines())
	}
}

func TestSearchAllDeduplicatesFamilyWithoutPagination(t *testing.T) {
	for _, missingPaging := range []string{appslack.SearchKindMessages, appslack.SearchKindFiles} {
		t.Run(missingPaging, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/search.all" {
					t.Errorf("unexpected per-result request: %s", r.URL.Path)
				}
				page := r.FormValue("page")
				if page == "" {
					page = "1"
				}
				messageID, fileID := page, page
				if missingPaging == appslack.SearchKindMessages {
					messageID = "1"
				} else {
					fileID = "1"
				}
				messages := map[string]any{"total": 2, "matches": []map[string]any{{"channel": map[string]any{"id": "C1"}, "ts": messageID, "text": "message"}}}
				files := map[string]any{"total": 2, "matches": []map[string]any{{"id": "F" + fileID}}}
				if missingPaging == appslack.SearchKindMessages {
					files["pagination"] = map[string]any{"page_count": 2}
				} else {
					messages["paging"] = map[string]any{"pages": 2}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": messages, "files": files})
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := NewService(client).Search(context.Background(), Params{Kind: appslack.SearchKindAll, Query: "synthetic", Count: 1, All: true})
			if err != nil {
				t.Fatal(err)
			}
			wantMessages, wantFiles := 2, 1
			if missingPaging == appslack.SearchKindMessages {
				wantMessages, wantFiles = 1, 2
			}
			if calls != 2 || len(result.Messages.Messages.Matches) != wantMessages || len(result.Files.Matches) != wantFiles {
				t.Errorf("duplicate results hide missing matches: messages=%d files=%d calls=%d", len(result.Messages.Messages.Matches), len(result.Files.Matches), calls)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["incomplete"] != true || fields["returned_count"] != float64(3) || fields["total_count"] != float64(4) {
				t.Errorf("duplicates concealed missing result: %s", encoded)
			}
			if !strings.Contains(strings.Join(result.Lines(), "\n"), "Incomplete results") {
				t.Errorf("human output hid missing result: %v", result.Lines())
			}
		})
	}
}

func TestSearchKeepsMessagesWithEqualTimestampsInDifferentChannels(t *testing.T) {
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{
		{Messages: appslack.SearchMessages{Total: 2, Matches: []appslack.SearchMatch{{Channel: appslack.SearchChannel{ID: "C1"}, Timestamp: "1.0"}}}, PageCount: 2, TotalCount: 2},
		{Messages: appslack.SearchMessages{Total: 2, Matches: []appslack.SearchMatch{{Channel: appslack.SearchChannel{ID: "C2"}, Timestamp: "1.0"}}}, PageCount: 2, TotalCount: 2},
	}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindMessages, Query: "x", All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages.Messages.Matches) != 2 {
		t.Fatalf("different channels were deduplicated: %#v", result.Messages.Messages.Matches)
	}
}

func TestSearchKeepsResultsWithoutStableIDs(t *testing.T) {
	page := appslack.UnifiedSearchResult{
		Messages: appslack.SearchMessages{Total: 4, Matches: []appslack.SearchMatch{
			{Timestamp: "1.0"}, {Channel: appslack.SearchChannel{ID: "C1"}},
		}},
		Files:     appslack.SearchFiles{Total: 2, Matches: []slackapi.File{{Title: "No ID"}}},
		PageCount: 2, TotalCount: 6,
	}
	mock := &mockSearchClient{pages: []appslack.UnifiedSearchResult{page, page}}
	result, err := NewService(mock).Search(context.Background(), Params{Kind: appslack.SearchKindAll, Query: "x", All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages.Messages.Matches) != 4 || len(result.Files.Matches) != 2 {
		t.Fatalf("results without stable identity were deduplicated: %#v", result)
	}
}

func TestSearchIncompleteIsCheckedPerFamily(t *testing.T) {
	result := Result{Kind: appslack.SearchKindAll, TotalCount: 3,
		Messages: &appslack.SearchResult{Messages: appslack.SearchMessages{Total: 1, Matches: []appslack.SearchMatch{{Text: "one"}, {Text: "two"}}}},
		Files:    &FileMatches{Total: 2, Matches: []slackapi.File{{ID: "F1"}}},
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["incomplete"] != true || !strings.Contains(strings.Join(result.Lines(), "\n"), "Incomplete results") {
		t.Fatalf("extra message hid missing file: %s", encoded)
	}
}
