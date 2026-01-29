package slack

import (
	"context"
	"errors"
	"testing"
	"time"

	slackapi "github.com/slack-go/slack"
)

func TestDoWithRetryStopsOnSuccess(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	err := DoWithRetry(ctx, func() error {
		callCount++
		if callCount == 2 {
			return nil
		}
		return errors.New("temporary")
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected two attempts, got %d", callCount)
	}
}

func TestDoWithRetryRateLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	called := 0
	err := DoWithRetry(ctx, func() error {
		called++
		return &slackapi.RateLimitedError{RetryAfter: time.Millisecond * 100}
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if called == 0 {
		t.Fatalf("expected retry attempts")
	}
}

func TestSearchParamsValidation(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantError bool
	}{
		{
			name:      "valid query",
			query:     "deployment failed",
			wantError: false,
		},
		{
			name:      "empty query",
			query:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewUserClient("xoxp-test-token")
			ctx := context.Background()
			_, err := client.SearchMessages(ctx, tt.query, SearchParams{
				Count:   20,
				Page:    1,
				SortBy:  "timestamp",
				SortDir: "desc",
			})

			// We expect an API error since we're using a fake token,
			// but we should not get a validation error for valid queries
			if tt.wantError && err == nil {
				t.Fatal("expected error for empty query, got nil")
			}
			if !tt.wantError && err != nil && err.Error() == "search query is required" {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestSearchResultLines(t *testing.T) {
	result := &SearchResult{
		Query: "test query",
		Messages: SearchMessages{
			Total: 2,
			Matches: []SearchMatch{
				{
					Type: "message",
					Channel: SearchChannel{
						ID:   "C123",
						Name: "general",
					},
					User:      "U123",
					Username:  "alice",
					Timestamp: "1705312365.000100",
					Text:      "This is a test message",
					Permalink: "https://example.slack.com/archives/C123/p1705312365000100",
				},
				{
					Type: "message",
					Channel: SearchChannel{
						ID:   "C456",
						Name: "devops",
					},
					User:      "U456",
					Username:  "bob",
					Timestamp: "1705312400.000200",
					Text:      "Another test message",
					Permalink: "https://example.slack.com/archives/C456/p1705312400000200",
				},
			},
		},
	}

	lines := result.Lines()
	if len(lines) == 0 {
		t.Fatal("expected non-empty lines output")
	}

	// Check header
	if lines[0] != "Search Results for \"test query\" (2 matches)" {
		t.Errorf("unexpected header: %s", lines[0])
	}

	// Verify we have content for both matches
	foundGeneral := false
	foundDevops := false
	for _, line := range lines {
		if contains(line, "#general") && contains(line, "@alice") {
			foundGeneral = true
		}
		if contains(line, "#devops") && contains(line, "@bob") {
			foundDevops = true
		}
	}

	if !foundGeneral {
		t.Error("expected to find general channel in output")
	}
	if !foundDevops {
		t.Error("expected to find devops channel in output")
	}
}

func TestSearchResultLinesEmpty(t *testing.T) {
	result := &SearchResult{
		Query: "no results",
		Messages: SearchMessages{
			Total:   0,
			Matches: []SearchMatch{},
		},
	}

	lines := result.Lines()
	if len(lines) < 3 {
		t.Fatal("expected at least header and no results message")
	}

	foundNoResults := false
	for _, line := range lines {
		if contains(line, "No messages found") {
			foundNoResults = true
			break
		}
	}

	if !foundNoResults {
		t.Error("expected 'No messages found' in output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
