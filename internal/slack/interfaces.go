package slack

import (
	"context"

	slackapi "github.com/slack-go/slack"
)

// Client defines the subset of Slack operations used by the CLI.
type Client interface {
	ListConversationsHistory(ctx context.Context, params HistoryParams) (*slackapi.GetConversationHistoryResponse, error)
	ListThreadReplies(ctx context.Context, params ThreadParams) ([]slackapi.Message, bool, string, error)
}

// ChannelClient extends Client with channel operations.
type ChannelClient interface {
	Client
	ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error)
	JoinChannel(ctx context.Context, channelID string) (*ChannelJoinResult, error)
	LeaveChannel(ctx context.Context, channelID string) (*ChannelLeaveResult, error)
}
