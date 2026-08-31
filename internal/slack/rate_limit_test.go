package slack

import (
	"context"
	"errors"
	"testing"
	"time"

	slackapi "github.com/slack-go/slack"
)

func TestRetryRateLimitedRetriesAndStops(t *testing.T) {
	attempts := 0
	got, err := RetryRateLimited(context.Background(), 2, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &slackapi.RateLimitedError{RetryAfter: time.Nanosecond}
		}
		return "ok", nil
	})
	if err != nil || got != "ok" || attempts != 3 {
		t.Fatalf("got=%q attempts=%d err=%v", got, attempts, err)
	}

	want := errors.New("other")
	_, err = RetryRateLimited(context.Background(), 3, func() (string, error) { return "", want })
	if !errors.Is(err, want) {
		t.Fatalf("expected non-rate-limit error, got %v", err)
	}
}

func TestRetryRateLimitedRejectsNegativeRetries(t *testing.T) {
	if _, err := RetryRateLimited(context.Background(), -1, func() (string, error) { return "", nil }); err == nil {
		t.Fatal("expected validation error")
	}
}
