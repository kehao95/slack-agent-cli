package slack

import (
	"context"
	"fmt"
	"strings"
)

// AuthTest verifies the user token is valid.
func (c *APIClient) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	resp, err := c.sdk.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}
	username := ""
	if name := strings.TrimSpace(resp.User); name != "" && name != resp.UserID {
		username = "@" + strings.TrimPrefix(name, "@")
	}
	return &AuthTestResponse{
		OK:       true,
		URL:      resp.URL,
		Team:     resp.Team,
		User:     resp.UserID,
		Username: username,
		TeamID:   resp.TeamID,
		UserID:   resp.UserID,
		BotID:    resp.BotID,
		IsBot:    resp.BotID != "",
	}, nil
}
