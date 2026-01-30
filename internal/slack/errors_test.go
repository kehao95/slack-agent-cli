package slack

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Ensure all sentinel errors are unique
	errs := []error{
		ErrChannelRequired,
		ErrTextRequired,
		ErrTimestampRequired,
		ErrEmojiRequired,
		ErrUserRequired,
		ErrQueryRequired,
		ErrNotFound,
		ErrRateLimited,
		ErrUnauthorized,
		ErrPermissionDenied,
	}

	for i, e1 := range errs {
		for j, e2 := range errs {
			if i != j && errors.Is(e1, e2) {
				t.Errorf("errors %v and %v should not be equal", e1, e2)
			}
		}
	}
}

func TestSentinelErrors_CanBeWrapped(t *testing.T) {
	wrapped := fmt.Errorf("operation failed: %w", ErrChannelRequired)

	if !errors.Is(wrapped, ErrChannelRequired) {
		t.Error("wrapped error should match ErrChannelRequired")
	}
}

func TestSentinelErrors_HaveDescriptiveMessages(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{ErrChannelRequired, "channel"},
		{ErrTextRequired, "text"},
		{ErrTimestampRequired, "timestamp"},
		{ErrEmojiRequired, "emoji"},
		{ErrUserRequired, "user"},
		{ErrQueryRequired, "query"},
		{ErrNotFound, "not found"},
		{ErrRateLimited, "rate"},
		{ErrUnauthorized, "unauthorized"},
		{ErrPermissionDenied, "permission"},
	}

	for _, tt := range tests {
		if !strings.Contains(tt.err.Error(), tt.contains) {
			t.Errorf("error %v should contain %q", tt.err, tt.contains)
		}
	}
}
