package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestSearchPreservesStructuredContent(t *testing.T) {
	// Synthetic alert: the text is intentionally empty and the useful content
	// only exists in its attachment. Include more than a short preview's worth.
	alertText := strings.Repeat("GPU worker unavailable. ", 400)
	for _, kind := range []string{SearchKindMessages, SearchKindAll} {
		for _, raw := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/resolved", true: "/raw"}[raw], func(t *testing.T) {
				t.Setenv("SLACK_CLI_READ_ONLY", "true")
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.URL.Path != "/search."+kind {
						t.Errorf("unexpected extra lookup: %s", r.URL.Path)
					}
					writeJSON(t, w, map[string]any{
						"ok": true,
						"messages": map[string]any{
							"total": 2, "paging": map[string]any{"page": 1, "pages": 1},
							"matches": []map[string]any{
								{"type": "message", "text": "", "ts": "1.0", "permalink": "https://example.test/alert", "channel": map[string]any{"id": "C1", "name": "alerts"},
									"attachments": []map[string]any{{"title": "GPU capacity", "title_link": "https://example.test/dashboard", "text": alertText, "fallback": "Capacity alert", "fields": []map[string]any{{"title": "Cluster", "value": "Synthetic cluster", "short": true}}}}},
								{"type": "message", "text": "", "ts": "2.0", "permalink": "https://example.test/blocks", "channel": map[string]any{"id": "C1", "name": "alerts"},
									"blocks": []map[string]any{
										{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "See <https://example.test/runbook|runbook>"}},
										{"type": "future_fixture_block", "payload": map[string]any{"link": "https://example.test/detail", "text": "Future block detail"}},
									}},
							},
						},
					})
				}))
				defer server.Close()
				client := New("xoxp-synthetic", slackapi.OptionAPIURL(server.URL+"/"))
				page, err := client.SearchResources(context.Background(), kind, "capacity", SearchParams{Count: 20, Page: 1})
				if err != nil {
					t.Fatal(err)
				}
				result := &SearchResult{Query: page.Query, Messages: page.Messages}
				result.SetRawJSON(raw)
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var output struct {
					Messages struct {
						Matches []struct {
							Text        string                   `json:"text"`
							Attachments []slackapi.Attachment    `json:"attachments"`
							Blocks      []map[string]interface{} `json:"blocks"`
						} `json:"matches"`
					} `json:"messages"`
				}
				if err := json.Unmarshal(encoded, &output); err != nil {
					t.Fatal(err)
				}
				if len(output.Messages.Matches) != 2 {
					t.Fatalf("unexpected matches: %s", encoded)
				}
				alert, blocks := output.Messages.Matches[0], output.Messages.Matches[1]
				if len(alert.Attachments) != 1 {
					t.Error("attachment-only alert lost its attachment")
				} else if alert.Attachments[0].Text != alertText || alert.Attachments[0].TitleLink != "https://example.test/dashboard" || len(alert.Attachments[0].Fields) != 1 || alert.Attachments[0].Fields[0].Value != "Synthetic cluster" {
					t.Error("attachment content, link, or fields changed")
				}
				if len(blocks.Blocks) != 2 {
					t.Error("block-only message lost its blocks")
				} else if blocks.Blocks[1]["payload"].(map[string]interface{})["link"] != "https://example.test/detail" {
					t.Error("unknown block content was not preserved")
				}
				if alert.Text != "" || blocks.Text != "" {
					t.Error("native text was replaced by synthesized content")
				}
				human := strings.Join(result.Lines(), "\n")
				for _, want := range []string{alertText, "GPU capacity", "Synthetic cluster", "https://example.test/dashboard", "https://example.test/runbook", "https://example.test/alert", "default JSON output"} {
					if !strings.Contains(human, want) {
						t.Errorf("human output omits %q", want[:min(len(want), 80)])
					}
				}
				if calls != 1 {
					t.Errorf("search made %d requests; want exactly one", calls)
				}
			})
		}
	}
}

func TestSearchResultReportsPartialPage(t *testing.T) {
	var result SearchResult
	if err := json.Unmarshal([]byte(`{"query":"capacity","messages":{"total":25,"matches":[{"text":"first"}]},"page":1,"page_count":3,"next_page":2,"has_more":true}`), &result); err != nil {
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
	human := strings.Join(result.Lines(), "\n")
	if !strings.Contains(human, "1 of 25") || !strings.Contains(human, "--page 2") {
		t.Errorf("human output hides partial results: %s", human)
	}
}
