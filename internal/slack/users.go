package slack

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"
)

// GetUserInfo fetches a single user's info.
func (c *APIClient) GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error) {
	user, err := c.sdk.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	return user, nil
}

// ListUsers fetches users with pagination using slack-go's pagination API.
// Note: slack-go doesn't expose cursor directly, so we fetch one page at a time
// using GetUsers with limit. The cursor parameter is ignored for now.
func (c *APIClient) ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error) {
	// slack-go's GetUsers doesn't support cursor-based pagination in the same way.
	// We use GetUsersPaginated iterator but fetch one page at a time.
	// For simplicity, fetch all users in one call (the SDK handles pagination internally).
	// This is a limitation - for very large workspaces, consider using the raw API.
	users, err := c.sdk.GetUsersContext(ctx, slackapi.GetUsersOptionLimit(limit))
	if err != nil {
		return nil, "", fmt.Errorf("list users: %w", err)
	}
	// Return empty cursor since we fetched all
	return users, "", nil
}

// GetUserGroups fetches all user groups from the workspace.
func (c *APIClient) GetUserGroups(ctx context.Context) ([]slackapi.UserGroup, error) {
	groups, err := c.sdk.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	return groups, nil
}

// GetUserPresence fetches the presence status of a specific user.
func (c *APIClient) GetUserPresence(ctx context.Context, userID string) (*slackapi.UserPresence, error) {
	presence, err := c.sdk.GetUserPresenceContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user presence: %w", err)
	}
	return presence, nil
}
