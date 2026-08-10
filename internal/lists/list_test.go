package lists

import (
	"context"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/slack"
)

type mockFetcher struct {
	listResp *slack.ListItemsResponse
	listErr  error
	listGot  []slack.ListItemsParams
	listFunc func(context.Context, slack.ListItemsParams) (*slack.ListItemsResponse, error)

	listInfoResp *slack.ListInfoResponse
	listInfoErr  error
	listInfoGot  []string

	itemResp *slack.ListItemInfoResponse
	itemErr  error
	itemGot  slack.ListItemInfoParams
}

type mockUserResolver struct {
	display map[string]string
	mention map[string]string
}

func (m *mockFetcher) ListSlackListItems(_ context.Context, params slack.ListItemsParams) (*slack.ListItemsResponse, error) {
	m.listGot = append(m.listGot, params)
	if m.listFunc != nil {
		return m.listFunc(context.Background(), params)
	}
	return m.listResp, m.listErr
}

func (m *mockFetcher) GetSlackList(_ context.Context, listID string) (*slack.ListInfoResponse, error) {
	m.listInfoGot = append(m.listInfoGot, listID)
	return m.listInfoResp, m.listInfoErr
}

func (m *mockFetcher) GetSlackListItem(_ context.Context, params slack.ListItemInfoParams) (*slack.ListItemInfoResponse, error) {
	m.itemGot = params
	return m.itemResp, m.itemErr
}

func (m mockUserResolver) GetDisplayName(_ context.Context, userID string) string {
	if value, ok := m.display[userID]; ok {
		return value
	}
	return userID
}

func (m mockUserResolver) GetMentionName(_ context.Context, userID string) string {
	if value, ok := m.mention[userID]; ok {
		return value
	}
	return userID
}

func TestResolveListID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw list id", input: "F0BFMJY6ZTQ", want: "F0BFMJY6ZTQ"},
		{name: "list url", input: "https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ", want: "F0BFMJY6ZTQ"},
		{name: "list url with trailing slash", input: "https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ/", want: "F0BFMJY6ZTQ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveListID(tt.input)
			if err != nil {
				t.Fatalf("ResolveListID returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveListIDRejectsInvalidInput(t *testing.T) {
	if _, err := ResolveListID("https://contentsquare.slack.com/archives/C123/p123"); err == nil {
		t.Fatal("expected invalid list reference to fail")
	}
}

func TestServiceListItems(t *testing.T) {
	fetcher := &mockFetcher{
		listResp: &slack.ListItemsResponse{
			OK: true,
			Items: []map[string]interface{}{
				{
					"id": "Ri123",
					"fields": []interface{}{
						map[string]interface{}{"text": "Helpdesk request A"},
					},
				},
			},
		},
		listInfoResp: &slack.ListInfoResponse{
			OK: true,
			File: map[string]interface{}{
				"id":    "F0BFMJY6ZTQ",
				"title": "Helpdesk",
			},
		},
	}
	fetcher.listResp.ResponseMetadata.NextCursor = "next"

	service := NewService(fetcher)
	result, err := service.ListItems(context.Background(), Params{
		List:   "https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ",
		Limit:  25,
		Cursor: "cursor",
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if len(fetcher.listGot) != 1 || fetcher.listGot[0].ListID != "F0BFMJY6ZTQ" {
		t.Fatalf("expected resolved list id, got %+v", fetcher.listGot)
	}
	if len(fetcher.listInfoGot) != 1 || fetcher.listInfoGot[0] != "F0BFMJY6ZTQ" {
		t.Fatalf("expected list metadata fetch, got %+v", fetcher.listInfoGot)
	}
	if fetcher.listGot[0].Limit != 25 || fetcher.listGot[0].Cursor != "cursor" {
		t.Fatalf("unexpected fetch params: %+v", fetcher.listGot[0])
	}
	if result.ListID != "F0BFMJY6ZTQ" || len(result.Items) != 1 || result.NextCursor != "next" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.List["title"] != "Helpdesk" {
		t.Fatalf("expected list metadata on result, got %+v", result.List)
	}
}

func TestServiceListItemsAllPages(t *testing.T) {
	responses := []*slack.ListItemsResponse{
		{
			OK: true,
			Items: []map[string]interface{}{
				{"id": "Ri1", "fields": []interface{}{map[string]interface{}{"text": "A"}}},
			},
		},
		{
			OK: true,
			Items: []map[string]interface{}{
				{"id": "Ri2", "fields": []interface{}{map[string]interface{}{"text": "B"}}},
			},
		},
	}
	responses[0].ResponseMetadata.NextCursor = "next"

	call := 0
	fetcher := &mockFetcher{
		listFunc: func(_ context.Context, _ slack.ListItemsParams) (*slack.ListItemsResponse, error) {
			resp := responses[call]
			call++
			return resp, nil
		},
	}

	service := NewService(fetcher)
	result, err := service.ListItems(context.Background(), Params{
		List:  "F0BFMJY6ZTQ",
		Limit: 100,
		All:   true,
	})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if len(fetcher.listGot) != 2 {
		t.Fatalf("expected 2 API calls, got %d", len(fetcher.listGot))
	}
	if fetcher.listGot[1].Cursor != "next" {
		t.Fatalf("expected second page cursor 'next', got %+v", fetcher.listGot[1])
	}
	if len(result.Items) != 2 || result.NextCursor != "" {
		t.Fatalf("unexpected paginated result: %+v", result)
	}
}

func TestGetItem(t *testing.T) {
	fetcher := &mockFetcher{
		itemResp: &slack.ListItemInfoResponse{
			OK: true,
			List: map[string]interface{}{
				"id":    "F0BFMJY6ZTQ",
				"title": "Helpdesk",
			},
			Record: map[string]interface{}{
				"id": "Rec123",
			},
		},
	}

	service := NewService(fetcher)
	result, err := service.GetItem(context.Background(), ItemParams{
		List:                "https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ",
		ID:                  "Rec123",
		IncludeIsSubscribed: true,
	})
	if err != nil {
		t.Fatalf("GetItem returned error: %v", err)
	}
	if fetcher.itemGot.ListID != "F0BFMJY6ZTQ" || fetcher.itemGot.ID != "Rec123" || !fetcher.itemGot.IncludeIsSubscribed {
		t.Fatalf("unexpected item fetch params: %+v", fetcher.itemGot)
	}
	if result.Record["id"] != "Rec123" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResultLinesUseSchemaAndUserResolution(t *testing.T) {
	result := Result{
		ListID: "F0BFMJY6ZTQ",
		List: map[string]interface{}{
			"title": "VOC Helpdesk tracker",
			"list_metadata": map[string]interface{}{
				"schema": []interface{}{
					map[string]interface{}{
						"id":                "ColRequest",
						"name":              "Request",
						"key":               "name",
						"type":              "text",
						"is_primary_column": true,
					},
					map[string]interface{}{
						"id":   "ColStatus",
						"name": "Status",
						"key":  "status",
						"type": "select",
						"options": map[string]interface{}{
							"choices": []interface{}{
								map[string]interface{}{"value": "done", "label": "Done"},
							},
						},
					},
					map[string]interface{}{
						"id":   "ColAssignee",
						"name": "Assignee",
						"key":  "assignee",
						"type": "user",
					},
				},
			},
		},
		Items: []map[string]interface{}{
			{
				"id": "Ri123",
				"fields": []interface{}{
					map[string]interface{}{"column_id": "ColAssignee", "user": []interface{}{"U1"}},
					map[string]interface{}{"column_id": "ColRequest", "text": "Helpdesk request A"},
					map[string]interface{}{"column_id": "ColStatus", "select": []interface{}{"done"}},
				},
			},
		},
		ctx: context.Background(),
		resolver: mockUserResolver{
			display: map[string]string{"U1": "Alice Example"},
			mention: map[string]string{"U1": "alice"},
		},
	}

	lines := result.Lines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	if lines[0] != "List VOC Helpdesk tracker - 1 items" {
		t.Fatalf("unexpected title line: %q", lines[0])
	}
	if lines[2] != "Ri123: Helpdesk request A | Done" {
		t.Fatalf("unexpected summary line: %q", lines[2])
	}
}

func TestItemResultLines(t *testing.T) {
	result := ItemResult{
		List: map[string]interface{}{
			"id":        "F0BFMJY6ZTQ",
			"title":     "Helpdesk",
			"permalink": "https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ",
			"list_metadata": map[string]interface{}{
				"schema": []interface{}{
					map[string]interface{}{
						"id":                "Col1",
						"name":              "Request",
						"key":               "name",
						"type":              "text",
						"is_primary_column": true,
					},
					map[string]interface{}{
						"id":   "Col2",
						"name": "Status",
						"key":  "status",
						"type": "select",
						"options": map[string]interface{}{
							"choices": []interface{}{
								map[string]interface{}{"value": "open", "label": "Open"},
							},
						},
					},
					map[string]interface{}{
						"id":   "Col3",
						"name": "Assignee",
						"key":  "assignee",
						"type": "user",
					},
				},
			},
		},
		Record: map[string]interface{}{
			"id":                "Rec123",
			"date_created":      float64(1758744346),
			"created_by":        "U1",
			"updated_timestamp": "1758744347",
			"updated_by":        "U2",
			"is_subscribed":     true,
			"fields": []interface{}{
				map[string]interface{}{"column_id": "Col1", "text": "Helpdesk request A"},
				map[string]interface{}{"column_id": "Col2", "select": []interface{}{"open"}},
				map[string]interface{}{"column_id": "Col3", "user": []interface{}{"U2"}},
			},
		},
		ctx: context.Background(),
		resolver: mockUserResolver{
			display: map[string]string{"U1": "Submitter", "U2": "Assignee Name"},
			mention: map[string]string{"U1": "submitter", "U2": "assignee"},
		},
	}

	lines := result.Lines()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"List: Helpdesk",
		"List ID: F0BFMJY6ZTQ",
		"List URL: https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ",
		"Created By: @submitter",
		"Updated By: @assignee",
		"- Request: Helpdesk request A",
		"- Status: Open",
		"- Assignee: @assignee",
		"Subscribed: true",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected output to contain %q, got %s", want, joined)
		}
	}
}
