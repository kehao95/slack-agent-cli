package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// UserProfileMutationResult is the response returned by users.profile.set.
// Profile is intentionally the native Slack profile shape so custom field IDs
// and values remain opaque to the CLI.
type UserProfileMutationResult struct {
	OK      bool                 `json:"ok"`
	Profile slackapi.UserProfile `json:"profile"`
}

// SetUserProfileFields updates only the fields present in profile. Callers
// should use this method for partial updates; serializing a zero-valued
// slack.UserProfile can clear fields that were not intended to change.
func (c *APIClient) SetUserProfileFields(ctx context.Context, userID string, profile map[string]interface{}) (*UserProfileMutationResult, error) {
	if len(profile) == 0 {
		return nil, fmt.Errorf("at least one profile field is required")
	}
	payload := map[string]interface{}{"profile": profile}
	if strings.TrimSpace(userID) != "" {
		payload["user"] = strings.TrimSpace(userID)
	}
	response, err := c.CallAPI(ctx, "users.profile.set", payload, CallAPIOptions{})
	if err != nil {
		return nil, fmt.Errorf("set user profile: %w", err)
	}
	var result UserProfileMutationResult
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode users.profile.set response: %w", err)
	}
	return &result, nil
}

// SetUserPresence changes the authenticated user's manual presence.
func (c *APIClient) SetUserPresence(ctx context.Context, presence string) error {
	presence = strings.ToLower(strings.TrimSpace(presence))
	if presence != "auto" && presence != "away" {
		return fmt.Errorf("presence must be auto or away")
	}
	if err := c.sdk.SetUserPresenceContext(ctx, presence); err != nil {
		return fmt.Errorf("set user presence: %w", err)
	}
	return nil
}

// SetUserPhoto changes the authenticated user's profile photo. The SDK's
// multipart helper is used so its guarded HTTP transport still sees the
// users.setPhoto method.
func (c *APIClient) SetUserPhoto(ctx context.Context, imagePath string, params slackapi.UserSetPhotoParams) error {
	if strings.TrimSpace(imagePath) == "" {
		return fmt.Errorf("image path is required")
	}
	if err := c.sdk.SetUserPhotoContext(ctx, imagePath, params); err != nil {
		return fmt.Errorf("set user photo: %w", err)
	}
	return nil
}

// DeleteUserPhoto removes the authenticated user's profile photo.
func (c *APIClient) DeleteUserPhoto(ctx context.Context) error {
	if err := c.sdk.DeleteUserPhotoContext(ctx); err != nil {
		return fmt.Errorf("delete user photo: %w", err)
	}
	return nil
}

// GetDNDInfo returns Do Not Disturb state for the current user or userID.
func (c *APIClient) GetDNDInfo(ctx context.Context, userID, teamID string) (*slackapi.DNDStatus, error) {
	var user *string
	if strings.TrimSpace(userID) != "" {
		value := strings.TrimSpace(userID)
		user = &value
	}
	options := make([]slackapi.ParamOption, 0, 1)
	if strings.TrimSpace(teamID) != "" {
		options = append(options, slackapi.DNDOptionTeamID(strings.TrimSpace(teamID)))
	}
	status, err := c.sdk.GetDNDInfoContext(ctx, user, options...)
	if err != nil {
		return nil, fmt.Errorf("get DND info: %w", err)
	}
	return status, nil
}

// GetDNDTeamInfo returns DND state for up to Slack's supported batch size.
func (c *APIClient) GetDNDTeamInfo(ctx context.Context, userIDs []string, teamID string) (map[string]slackapi.DNDStatus, error) {
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("at least one user is required")
	}
	if len(userIDs) > 50 {
		return nil, fmt.Errorf("at most 50 users may be requested")
	}
	options := make([]slackapi.ParamOption, 0, 1)
	if strings.TrimSpace(teamID) != "" {
		options = append(options, slackapi.DNDOptionTeamID(strings.TrimSpace(teamID)))
	}
	status, err := c.sdk.GetDNDTeamInfoContext(ctx, userIDs, options...)
	if err != nil {
		return nil, fmt.Errorf("get team DND info: %w", err)
	}
	return status, nil
}

// SetDNDSnooze starts or changes the authenticated user's DND snooze.
func (c *APIClient) SetDNDSnooze(ctx context.Context, minutes int) (*slackapi.DNDStatus, error) {
	if minutes <= 0 {
		return nil, fmt.Errorf("minutes must be greater than zero")
	}
	status, err := c.sdk.SetSnoozeContext(ctx, minutes)
	if err != nil {
		return nil, fmt.Errorf("set DND snooze: %w", err)
	}
	return status, nil
}

// EndDND ends the authenticated user's scheduled DND session.
func (c *APIClient) EndDND(ctx context.Context) error {
	if err := c.sdk.EndDNDContext(ctx); err != nil {
		return fmt.Errorf("end DND: %w", err)
	}
	return nil
}

// EndDNDSnooze ends the authenticated user's current DND snooze.
func (c *APIClient) EndDNDSnooze(ctx context.Context) (*slackapi.DNDStatus, error) {
	status, err := c.sdk.EndSnoozeContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("end DND snooze: %w", err)
	}
	return status, nil
}

// ListConversationsPage exposes one conversations.list page for deterministic
// directory-backed conversation discovery.
func (c *APIClient) ListConversationsPage(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error) {
	return c.ListChannels(ctx, params)
}
