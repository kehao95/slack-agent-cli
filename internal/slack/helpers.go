package slack

import (
	"context"
	"fmt"
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
