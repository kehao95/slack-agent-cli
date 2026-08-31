package slack

import (
	"context"
	"fmt"
	"strings"

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

// ListUsers fetches exactly one page of users and returns Slack's continuation cursor.
func (c *APIClient) ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error) {
	if limit <= 0 {
		limit = 100
	}
	page := c.sdk.GetUsersPaginated(
		slackapi.GetUsersOptionLimit(limit),
		slackapi.GetUsersOptionCursor(strings.TrimSpace(cursor)),
	)
	page, err := page.Next(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("list users: %w", err)
	}
	return page.Users, page.Cursor, nil
}

// GetUserByEmail retrieves a user by their normalized email address.
func (c *APIClient) GetUserByEmail(ctx context.Context, email string) (*slackapi.User, error) {
	user, err := c.sdk.GetUserByEmailContext(ctx, strings.TrimSpace(email))
	if err != nil {
		return nil, fmt.Errorf("lookup user by email: %w", err)
	}
	return user, nil
}

// GetUserProfile retrieves a user's profile. An empty userID means the caller.
func (c *APIClient) GetUserProfile(ctx context.Context, userID string, includeLabels bool) (*slackapi.UserProfile, error) {
	profile, err := c.sdk.GetUserProfileContext(ctx, &slackapi.GetUserProfileParameters{
		UserID:        strings.TrimSpace(userID),
		IncludeLabels: includeLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	return profile, nil
}

// SetUserProfile replaces the supplied fields on a user's profile.
func (c *APIClient) SetUserProfile(ctx context.Context, userID string, profile *slackapi.UserProfile) error {
	if profile == nil {
		return fmt.Errorf("profile is required")
	}
	if err := c.sdk.SetUserProfileContext(ctx, strings.TrimSpace(userID), profile); err != nil {
		return fmt.Errorf("set user profile: %w", err)
	}
	return nil
}

// SetUserStatus updates or clears only the custom-status fields of a profile.
func (c *APIClient) SetUserStatus(ctx context.Context, userID, text, emoji string, expiration int64) error {
	if err := c.sdk.SetUserCustomStatusContextWithUser(ctx, strings.TrimSpace(userID), text, emoji, expiration); err != nil {
		return fmt.Errorf("set user status: %w", err)
	}
	return nil
}

// ListUserConversations fetches one page of conversations visible to a user.
func (c *APIClient) ListUserConversations(ctx context.Context, params slackapi.GetConversationsForUserParameters) ([]slackapi.Channel, string, error) {
	channels, cursor, err := c.sdk.GetConversationsForUserContext(ctx, &params)
	if err != nil {
		return nil, "", fmt.Errorf("list user conversations: %w", err)
	}
	return channels, cursor, nil
}

// GetUserGroups fetches all user groups from the workspace.
func (c *APIClient) GetUserGroups(ctx context.Context) ([]slackapi.UserGroup, error) {
	groups, err := c.sdk.GetUserGroupsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user groups: %w", err)
	}
	return groups, nil
}

type UserGroupListOptions struct {
	IncludeUsers    bool
	IncludeCount    bool
	IncludeDisabled bool
	TeamID          string
}

func (c *APIClient) ListUserGroups(ctx context.Context, opts UserGroupListOptions) ([]slackapi.UserGroup, error) {
	groups, err := c.sdk.GetUserGroupsContext(ctx,
		slackapi.GetUserGroupsOptionIncludeUsers(opts.IncludeUsers),
		slackapi.GetUserGroupsOptionIncludeCount(opts.IncludeCount),
		slackapi.GetUserGroupsOptionIncludeDisabled(opts.IncludeDisabled),
		slackapi.GetUserGroupsOptionTeamID(opts.TeamID),
	)
	if err != nil {
		return nil, fmt.Errorf("list user groups: %w", err)
	}
	return groups, nil
}

func (c *APIClient) GetUserGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	members, err := c.sdk.GetUserGroupMembersContext(ctx, strings.TrimSpace(groupID))
	if err != nil {
		return nil, fmt.Errorf("get user group members: %w", err)
	}
	return members, nil
}

func (c *APIClient) CreateUserGroup(ctx context.Context, group slackapi.UserGroup, includeCount, enableSection bool) (*slackapi.UserGroup, error) {
	created, err := c.sdk.CreateUserGroupContext(ctx, group, slackapi.CreateUserGroupOptionIncludeCount(includeCount), slackapi.CreateUserGroupOptionEnableSection(enableSection))
	if err != nil {
		return nil, fmt.Errorf("create user group: %w", err)
	}
	return &created, nil
}

type UserGroupUpdateOptions struct {
	Name          string
	Handle        string
	Description   *string
	Channels      *[]string
	EnableSection bool
	TeamID        string
}

func (c *APIClient) UpdateUserGroup(ctx context.Context, groupID string, opts UserGroupUpdateOptions) (*slackapi.UserGroup, error) {
	options := []slackapi.UpdateUserGroupsOption{slackapi.UpdateUserGroupsOptionTeamID(opts.TeamID)}
	if opts.Name != "" {
		options = append(options, slackapi.UpdateUserGroupsOptionName(opts.Name))
	}
	if opts.Handle != "" {
		options = append(options, slackapi.UpdateUserGroupsOptionHandle(opts.Handle))
	}
	if opts.Description != nil {
		options = append(options, slackapi.UpdateUserGroupsOptionDescription(opts.Description))
	}
	if opts.Channels != nil {
		options = append(options, slackapi.UpdateUserGroupsOptionChannels(*opts.Channels))
	}
	if opts.EnableSection {
		options = append(options, slackapi.UpdateUserGroupsOptionEnableSection(true))
	}
	updated, err := c.sdk.UpdateUserGroupContext(ctx, strings.TrimSpace(groupID), options...)
	if err != nil {
		return nil, fmt.Errorf("update user group: %w", err)
	}
	return &updated, nil
}

func (c *APIClient) SetUserGroupEnabled(ctx context.Context, groupID string, enabled bool, includeCount bool, teamID string) (*slackapi.UserGroup, error) {
	var group slackapi.UserGroup
	var err error
	if enabled {
		group, err = c.sdk.EnableUserGroupContext(ctx, groupID, slackapi.EnableUserGroupOptionIncludeCount(includeCount), slackapi.EnableUserGroupOptionTeamID(teamID))
	} else {
		group, err = c.sdk.DisableUserGroupContext(ctx, groupID, slackapi.DisableUserGroupOptionIncludeCount(includeCount), slackapi.DisableUserGroupOptionTeamID(teamID))
	}
	if err != nil {
		return nil, fmt.Errorf("set user group enabled: %w", err)
	}
	return &group, nil
}

func (c *APIClient) UpdateUserGroupMembers(ctx context.Context, groupID string, members []string) (*slackapi.UserGroup, error) {
	group, err := c.sdk.UpdateUserGroupMembersListContext(ctx, strings.TrimSpace(groupID), members)
	if err != nil {
		return nil, fmt.Errorf("update user group members: %w", err)
	}
	return &group, nil
}

// GetUserPresence fetches the presence status of a specific user.
func (c *APIClient) GetUserPresence(ctx context.Context, userID string) (*slackapi.UserPresence, error) {
	presence, err := c.sdk.GetUserPresenceContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user presence: %w", err)
	}
	return presence, nil
}
