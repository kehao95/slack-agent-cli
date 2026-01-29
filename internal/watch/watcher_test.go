package watch

import (
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	cfg := Config{
		Channels:       []string{"C123", "C456"},
		IncludeBots:    false,
		IncludeOwn:     false,
		IncludeThreads: true,
		Events:         []string{"message"},
		JSONOutput:     false,
		Quiet:          false,
		Timeout:        5 * time.Minute,
		BotUserID:      "U123",
	}

	if len(cfg.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(cfg.Channels))
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected 5m timeout, got %v", cfg.Timeout)
	}
}

func TestEvent(t *testing.T) {
	evt := Event{
		Type:        "message",
		Timestamp:   "1705312365.000100",
		Channel:     "C123ABC",
		ChannelName: "general",
		User:        "U456DEF",
		Username:    "alice",
		Text:        "Hello everyone!",
	}

	if evt.Type != "message" {
		t.Errorf("expected type message, got %s", evt.Type)
	}
	if evt.Username != "alice" {
		t.Errorf("expected username alice, got %s", evt.Username)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "valid timestamp",
			input: "1705312365.000100",
		},
		{
			name:  "timestamp without microseconds",
			input: "1705312365",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watcher{
				channelCache: make(map[string]string),
				userCache:    make(map[string]string),
			}
			result := w.formatTimestamp(tt.input)
			// Just check it's not empty and in expected format
			if result == "" {
				t.Errorf("expected non-empty timestamp")
			}
			// Check it matches the format YYYY-MM-DD HH:MM:SS
			if len(result) != 19 {
				t.Errorf("expected timestamp format YYYY-MM-DD HH:MM:SS (19 chars), got %s (%d chars)", result, len(result))
			}
		})
	}
}

func TestFormatHumanReadable(t *testing.T) {
	w := &Watcher{
		channelCache: make(map[string]string),
		userCache:    make(map[string]string),
	}

	tests := []struct {
		name     string
		event    Event
		contains string
	}{
		{
			name: "message event",
			event: Event{
				Type:        "message",
				Timestamp:   "1705312365.000100",
				Channel:     "C123",
				ChannelName: "general",
				User:        "U123",
				Username:    "alice",
				Text:        "Hello world",
			},
			contains: "@alice: Hello world",
		},
		{
			name: "reaction added",
			event: Event{
				Type:        "reaction_added",
				Timestamp:   "1705312365.000100",
				Channel:     "C123",
				ChannelName: "general",
				User:        "U123",
				Username:    "bob",
				Reaction:    "wave",
			},
			contains: "reacted with :wave:",
		},
		{
			name: "thread message",
			event: Event{
				Type:        "message",
				Timestamp:   "1705312365.000100",
				Channel:     "C123",
				ChannelName: "general",
				User:        "U123",
				Username:    "alice",
				Text:        "Thread reply",
				ThreadTS:    "1705312300.000100",
			},
			contains: "(thread)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := w.formatHumanReadable(tt.event)
			if result == "" {
				t.Errorf("expected non-empty result")
			}
			if !containsString(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestIsEventEnabled(t *testing.T) {
	tests := []struct {
		name      string
		events    []string
		eventType string
		expected  bool
	}{
		{
			name:      "empty config allows all",
			events:    []string{},
			eventType: "message",
			expected:  true,
		},
		{
			name:      "specific event enabled",
			events:    []string{"message", "reaction_added"},
			eventType: "message",
			expected:  true,
		},
		{
			name:      "event not in list",
			events:    []string{"message"},
			eventType: "reaction_added",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watcher{
				config: Config{
					Events: tt.events,
				},
			}
			result := w.isEventEnabled(tt.eventType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestShouldWatchChannel(t *testing.T) {
	tests := []struct {
		name      string
		channels  []string
		channelID string
		expected  bool
	}{
		{
			name:      "empty config watches all",
			channels:  []string{},
			channelID: "C123",
			expected:  true,
		},
		{
			name:      "channel in list",
			channels:  []string{"C123", "C456"},
			channelID: "C123",
			expected:  true,
		},
		{
			name:      "channel not in list",
			channels:  []string{"C123"},
			channelID: "C456",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watcher{
				config: Config{
					Channels: tt.channels,
				},
			}
			result := w.shouldWatchChannel(tt.channelID)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
