package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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

func readRequiredStdin(format string) (string, error) {
	text, err := readStdinIfPiped()
	if err != nil {
		return "", err
	}
	if text == "" {
		return "", fmt.Errorf("--%s - requires piped stdin", format)
	}
	return text, nil
}

// parseBlocksJSON parses a JSON array of Slack Block Kit blocks.
// Returns nil if blocksJSON is empty.
func parseBlocksJSON(blocksJSON string) ([]slackapi.Block, error) {
	if blocksJSON == "" {
		return nil, nil
	}

	resolved, err := readJSONArgument(blocksJSON)
	if err != nil {
		return nil, err
	}

	var rawBlocks []json.RawMessage
	if err := json.Unmarshal([]byte(resolved), &rawBlocks); err != nil {
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
		Type    string `json:"type"`
		BlockID string `json:"block_id"`
	}
	if err := json.Unmarshal(raw, &blockType); err != nil {
		return nil, fmt.Errorf("parse block type: %w", err)
	}

	if strings.TrimSpace(blockType.Type) == "" {
		return nil, fmt.Errorf("block type is required")
	}

	// Keep the original JSON instead of decoding into the SDK's current set of
	// concrete block structs. Slack adds Block Kit types independently of the Go
	// SDK release cadence; forwarding the validated object keeps this CLI
	// compatible with new block types and fields.
	return rawBlock{raw: append(json.RawMessage(nil), raw...), blockType: blockType.Type, blockID: blockType.BlockID}, nil
}

type rawBlock struct {
	raw       json.RawMessage
	blockType string
	blockID   string
}

func (b rawBlock) BlockType() slackapi.MessageBlockType {
	return slackapi.MessageBlockType(b.blockType)
}
func (b rawBlock) ID() string                   { return b.blockID }
func (b rawBlock) MarshalJSON() ([]byte, error) { return b.raw, nil }

// readJSONArgument accepts inline JSON, @path, or '-' for stdin. This is used
// for payload-shaped flags so agents can avoid shell escaping large objects.
func readJSONArgument(value string) (string, error) {
	switch {
	case value == "-":
		return readRequiredStdin("blocks")
	case strings.HasPrefix(value, "@"):
		path := strings.TrimSpace(strings.TrimPrefix(value, "@"))
		if path == "" {
			return "", fmt.Errorf("JSON file path is required after @")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read JSON file %s: %w", path, err)
		}
		return string(data), nil
	default:
		return value, nil
	}
}
