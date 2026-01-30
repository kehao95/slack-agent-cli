package slack

import (
	"context"
	"fmt"
	"time"

	slackapi "github.com/slack-go/slack"
)

// AuthTest verifies the user token is valid.
func (c *APIClient) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	resp, err := c.sdk.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}
	return &AuthTestResponse{
		OK:     true,
		URL:    resp.URL,
		Team:   resp.Team,
		User:   resp.User,
		TeamID: resp.TeamID,
		UserID: resp.UserID,
		BotID:  resp.BotID,
	}, nil
}

// DoWithRetry executes fn with simple retry logic for rate-limited operations.
func DoWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	backoff := time.Second
	for attempts := 0; attempts < 3; attempts++ {
		if err := fn(); err != nil {
			if rlErr, ok := err.(*slackapi.RateLimitedError); ok {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(rlErr.RetryAfter):
					lastErr = err
					continue
				}
			}
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return nil
	}
	return lastErr
}
