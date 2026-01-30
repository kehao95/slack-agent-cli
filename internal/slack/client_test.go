package slack

import (
	"context"
	"testing"

	slackapi "github.com/slack-go/slack"
)

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

func TestPostMessageValidation(t *testing.T) {
	tests := []struct {
		name      string
		channel   string
		opts      PostMessageOptions
		wantError string
	}{
		{
			name:      "valid message with text",
			channel:   "C123ABC",
			opts:      PostMessageOptions{Text: "Hello"},
			wantError: "",
		},
		{
			name:      "empty channel",
			channel:   "",
			opts:      PostMessageOptions{Text: "Hello"},
			wantError: "channel is required",
		},
		{
			name:      "no text or blocks",
			channel:   "C123ABC",
			opts:      PostMessageOptions{},
			wantError: "text or blocks is required",
		},
		{
			name:    "valid with blocks",
			channel: "C123ABC",
			opts: PostMessageOptions{
				Blocks: []slackapi.Block{
					slackapi.NewSectionBlock(nil, nil, nil),
				},
			},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New("xoxb-test-token")
			ctx := context.Background()

			_, err := client.PostMessage(ctx, tt.channel, tt.opts)

			if tt.wantError == "" {
				// We expect an API error since we're using a fake token,
				// but not a validation error
				if err != nil && err.Error() == tt.wantError {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else {
				// We expect a specific validation error
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %q", tt.wantError, err.Error())
				}
			}
		})
	}
}

func TestPostMessageResultLines(t *testing.T) {
	result := &PostMessageResult{
		OK:        true,
		Channel:   "#general",
		Timestamp: "1705312365.000100",
		Text:      "Test message",
	}

	lines := result.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	if !contains(lines[0], "Message sent successfully") {
		t.Errorf("expected success message in first line, got %q", lines[0])
	}

	if !contains(lines[1], "#general") {
		t.Errorf("expected channel in output, got %q", lines[1])
	}

	if !contains(lines[2], "1705312365.000100") {
		t.Errorf("expected timestamp in output, got %q", lines[2])
	}
}

func TestReactionValidation(t *testing.T) {
	tests := []struct {
		name      string
		channel   string
		timestamp string
		emoji     string
		wantError string
	}{
		{
			name:      "valid reaction",
			channel:   "C123ABC",
			timestamp: "1705312365.000100",
			emoji:     "thumbsup",
			wantError: "",
		},
		{
			name:      "empty channel",
			channel:   "",
			timestamp: "1705312365.000100",
			emoji:     "thumbsup",
			wantError: "channel is required",
		},
		{
			name:      "empty timestamp",
			channel:   "C123ABC",
			timestamp: "",
			emoji:     "thumbsup",
			wantError: "timestamp is required",
		},
		{
			name:      "empty emoji",
			channel:   "C123ABC",
			timestamp: "1705312365.000100",
			emoji:     "",
			wantError: "emoji is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (add)", func(t *testing.T) {
			client := New("xoxb-test-token")
			ctx := context.Background()

			err := client.AddReaction(ctx, tt.channel, tt.timestamp, tt.emoji)

			if tt.wantError == "" {
				// We expect an API error since we're using a fake token,
				// but not a validation error
				if err != nil && contains(err.Error(), "required") {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else {
				// We expect a specific validation error
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %q", tt.wantError, err.Error())
				}
			}
		})

		t.Run(tt.name+" (remove)", func(t *testing.T) {
			client := New("xoxb-test-token")
			ctx := context.Background()

			err := client.RemoveReaction(ctx, tt.channel, tt.timestamp, tt.emoji)

			if tt.wantError == "" {
				// We expect an API error since we're using a fake token,
				// but not a validation error
				if err != nil && contains(err.Error(), "required") {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else {
				// We expect a specific validation error
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %q", tt.wantError, err.Error())
				}
			}
		})
	}
}

func TestReactionResultLines(t *testing.T) {
	tests := []struct {
		name     string
		result   *ReactionResult
		expected string
	}{
		{
			name: "add reaction",
			result: &ReactionResult{
				OK:        true,
				Action:    "add",
				Channel:   "#general",
				ChannelID: "C123ABC",
				Timestamp: "1705312365.000100",
				Emoji:     "thumbsup",
			},
			expected: "✓ Added :thumbsup: to message in #general",
		},
		{
			name: "remove reaction",
			result: &ReactionResult{
				OK:        true,
				Action:    "remove",
				Channel:   "#devops",
				ChannelID: "C456DEF",
				Timestamp: "1705312365.000100",
				Emoji:     "rocket",
			},
			expected: "✓ Removed :rocket: from message in #devops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := tt.result.Lines()
			if len(lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(lines))
			}

			if lines[0] != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, lines[0])
			}
		})
	}
}
