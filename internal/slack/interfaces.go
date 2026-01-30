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

// MessageClient provides message posting capabilities.
type MessageClient interface {
	PostMessage(ctx context.Context, channel string, opts PostMessageOptions) (*PostMessageResult, error)
	EditMessage(ctx context.Context, channel, timestamp, text string) (*EditMessageResult, error)
	DeleteMessage(ctx context.Context, channel, timestamp string) (*DeleteMessageResult, error)
}

// ChannelClient extends Client with channel operations.
type ChannelClient interface {
	Client
	ListChannels(ctx context.Context, params ListChannelsParams) ([]slackapi.Channel, string, error)
	JoinChannel(ctx context.Context, channelID string) (*ChannelJoinResult, error)
	LeaveChannel(ctx context.Context, channelID string) (*ChannelLeaveResult, error)
}

// UserClient extends Client with user operations.
type UserClient interface {
	Client
	GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error)
	ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error)
}

// ReactionClient provides reaction capabilities.
type ReactionClient interface {
	AddReaction(ctx context.Context, channel, timestamp, emoji string) error
	RemoveReaction(ctx context.Context, channel, timestamp, emoji string) error
	GetReactions(ctx context.Context, channel, timestamp string) (*ReactionListResult, error)
}

// PinClient provides pin capabilities.
type PinClient interface {
	AddPin(ctx context.Context, channel, timestamp string) error
	RemovePin(ctx context.Context, channel, timestamp string) error
	ListPins(ctx context.Context, channel string) (*PinListResult, error)
}

// EmojiClient provides emoji capabilities.
type EmojiClient interface {
	ListEmoji(ctx context.Context) (*EmojiListResult, error)
}

// SearchClient provides message search capabilities (requires user token).
type SearchClient interface {
	SearchMessages(ctx context.Context, query string, params SearchParams) (*SearchResult, error)
}

// FullClient combines all client capabilities.
type FullClient interface {
	ChannelClient
	UserClient
}
