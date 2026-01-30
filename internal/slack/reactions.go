package slack

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"
)

// AddReaction adds an emoji reaction to a message.
func (c *APIClient) AddReaction(ctx context.Context, channel, timestamp, emoji string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.AddReactionContext(ctx, emoji, itemRef)
}

// RemoveReaction removes an emoji reaction from a message.
func (c *APIClient) RemoveReaction(ctx context.Context, channel, timestamp, emoji string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.RemoveReactionContext(ctx, emoji, itemRef)
}

// GetReactions retrieves all reactions on a specific message.
func (c *APIClient) GetReactions(ctx context.Context, channel, timestamp string) (*ReactionListResult, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return nil, fmt.Errorf("timestamp is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	params := slackapi.GetReactionsParameters{
		Full: true, // Get full details including user list
	}

	reactions, err := c.sdk.GetReactionsContext(ctx, itemRef, params)
	if err != nil {
		return nil, fmt.Errorf("get reactions: %w", err)
	}

	// Convert slack-go ItemReaction to our ReactionItem structure
	reactionItems := make([]ReactionItem, 0, len(reactions))
	for _, reaction := range reactions {
		reactionItems = append(reactionItems, ReactionItem{
			Name:  reaction.Name,
			Count: reaction.Count,
			Users: reaction.Users,
		})
	}

	return &ReactionListResult{
		OK:        true,
		Channel:   channel,
		ChannelID: channel,
		Timestamp: timestamp,
		Reactions: reactionItems,
	}, nil
}
