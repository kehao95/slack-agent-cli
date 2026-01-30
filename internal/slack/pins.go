package slack

import (
	"context"
	"fmt"

	slackapi "github.com/slack-go/slack"
)

// AddPin pins a message to a channel.
func (c *APIClient) AddPin(ctx context.Context, channel, timestamp string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.AddPinContext(ctx, channel, itemRef)
}

// RemovePin removes a pin from a message.
func (c *APIClient) RemovePin(ctx context.Context, channel, timestamp string) error {
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	itemRef := slackapi.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	return c.sdk.RemovePinContext(ctx, channel, itemRef)
}

// ListPins lists all pinned items in a channel.
func (c *APIClient) ListPins(ctx context.Context, channel string) (*PinListResult, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel is required")
	}

	items, _, err := c.sdk.ListPinsContext(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}

	// Convert slack-go Items to our PinnedItem structure
	pinnedItems := make([]PinnedItem, 0, len(items))
	for _, item := range items {
		pinnedItem := PinnedItem{
			Type:    item.Type,
			Channel: item.Channel,
		}

		// If it's a message, convert the message data
		if item.Message != nil {
			pinnedItem.Message = &Message{
				Timestamp: item.Message.Timestamp,
				Text:      item.Message.Text,
				User:      item.Message.User,
			}
		}

		pinnedItems = append(pinnedItems, pinnedItem)
	}

	return &PinListResult{
		OK:      true,
		Channel: channel,
		Items:   pinnedItems,
	}, nil
}
