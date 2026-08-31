package slack

import (
	"context"
	"errors"
	"time"

	slackapi "github.com/slack-go/slack"
)

// RetryRateLimited retries one Slack operation when the SDK reports HTTP 429.
// It is shared by resource pagination so every dedicated abstraction honors
// Retry-After consistently.
func RetryRateLimited[T any](ctx context.Context, maxRetries int, call func() (T, error)) (T, error) {
	var zero T
	if maxRetries < 0 {
		return zero, errors.New("max retries cannot be negative")
	}
	for attempt := 0; ; attempt++ {
		result, err := call()
		if err == nil {
			return result, nil
		}
		var rateLimited *slackapi.RateLimitedError
		if !errors.As(err, &rateLimited) || attempt >= maxRetries {
			return zero, err
		}
		delay := rateLimited.RetryAfter
		if delay <= 0 {
			delay = time.Second
		}
		if err := WaitContext(ctx, delay); err != nil {
			return zero, err
		}
	}
}

// WaitContext delays without ignoring cancellation or command deadlines.
func WaitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
