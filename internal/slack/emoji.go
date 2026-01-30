package slack

import (
	"context"
	"fmt"
)

// ListEmoji retrieves all custom emoji in the workspace.
func (c *APIClient) ListEmoji(ctx context.Context) (*EmojiListResult, error) {
	emoji, err := c.sdk.GetEmojiContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("list emoji: %w", err)
	}

	return &EmojiListResult{
		OK:    true,
		Emoji: emoji,
		Count: len(emoji),
	}, nil
}
