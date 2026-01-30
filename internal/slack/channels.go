package slack

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"
)

// ListChannels fetches channels the calling user is a member of.
// Uses users.conversations API which works with channels:read scope on user tokens.
// Note: private_channel type requires groups:read scope, im type requires im:read scope.
func (c *APIClient) ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error) {
	convParams := &slackapi.GetConversationsForUserParameters{
		Limit:           params.Limit,
		Cursor:          params.Cursor,
		ExcludeArchived: !params.IncludeArchived,
	}
	// Only set types if explicitly provided - this avoids scope issues
	// When no types are specified, the API defaults to public channels only
	if len(params.Types) > 0 {
		convParams.Types = append(convParams.Types, params.Types...)
	}
	channels, nextCursor, err := c.sdk.GetConversationsForUserContext(ctx, convParams)
	return channels, nextCursor, err
}

// ListChannelsPaginated provides a simpler interface for cache population.
// Returns public channels the user is a member of (uses users.conversations API).
// Note: Only fetches public_channel type to work with channels:read scope.
func (c *APIClient) ListChannelsPaginated(ctx context.Context, cursor string, limit int) ([]slackapi.Channel, string, int, error) {
	channels, nextCursor, err := c.ListChannels(ctx, ListChannelsParams{
		Limit:           limit,
		Cursor:          cursor,
		IncludeArchived: false,
		Types:           []string{"public_channel"},
	})
	if err != nil {
		return nil, "", 0, err
	}

	return channels, nextCursor, len(channels), nil
}

// JoinChannel joins a channel by ID.
func (c *APIClient) JoinChannel(ctx context.Context, channelID string) (*ChannelJoinResult, error) {
	if channelID == "" {
		return nil, ErrChannelRequired
	}

	channel, _, _, err := c.sdk.JoinConversationContext(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("join channel: %w", err)
	}

	channelName := channel.Name
	if channelName == "" {
		channelName = channelID
	}

	return &ChannelJoinResult{
		OK:        true,
		Channel:   channelName,
		ChannelID: channelID,
	}, nil
}

// LeaveChannel leaves a channel by ID.
func (c *APIClient) LeaveChannel(ctx context.Context, channelID string) (*ChannelLeaveResult, error) {
	if channelID == "" {
		return nil, ErrChannelRequired
	}

	_, err := c.sdk.LeaveConversationContext(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("leave channel: %w", err)
	}

	return &ChannelLeaveResult{
		OK:        true,
		Channel:   channelID,
		ChannelID: channelID,
	}, nil
}
