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
