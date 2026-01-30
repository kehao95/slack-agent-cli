package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	slackapi "github.com/slack-go/slack"
)

// readStdinIfPiped reads from stdin if data is being piped in.
// Returns empty string if stdin is a terminal (no piped data).
func readStdinIfPiped() (string, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
	return "", nil
}

// parseBlocksJSON parses a JSON array of Slack Block Kit blocks.
// Returns nil if blocksJSON is empty.
func parseBlocksJSON(blocksJSON string) ([]slackapi.Block, error) {
	if blocksJSON == "" {
		return nil, nil
	}

	var rawBlocks []json.RawMessage
	if err := json.Unmarshal([]byte(blocksJSON), &rawBlocks); err != nil {
		return nil, fmt.Errorf("invalid blocks JSON array: %w", err)
	}

	blocks := make([]slackapi.Block, 0, len(rawBlocks))
	for i, raw := range rawBlocks {
		block, err := parseBlock(raw)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// parseBlock parses a single Slack block from JSON.
func parseBlock(raw json.RawMessage) (slackapi.Block, error) {
	var blockType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &blockType); err != nil {
		return nil, fmt.Errorf("parse block type: %w", err)
	}

	switch blockType.Type {
	case "section":
		var b slackapi.SectionBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse section block: %w", err)
		}
		return &b, nil
	case "divider":
		var b slackapi.DividerBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse divider block: %w", err)
		}
		return &b, nil
	case "header":
		var b slackapi.HeaderBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse header block: %w", err)
		}
		return &b, nil
	case "context":
		var b slackapi.ContextBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse context block: %w", err)
		}
		return &b, nil
	case "actions":
		var b slackapi.ActionBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse actions block: %w", err)
		}
		return &b, nil
	case "image":
		var b slackapi.ImageBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("parse image block: %w", err)
		}
		return &b, nil
	default:
		return nil, fmt.Errorf("unsupported block type: %s", blockType.Type)
	}
}
