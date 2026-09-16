package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/output"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var messagesPreviewCmd = &cobra.Command{
	Use:     "preview",
	Short:   "Validate and inspect a local message payload",
	Long:    "Parse message text, Block Kit, and metadata locally. No Slack authentication or network request is made. Blocks are structurally checked but not rendered as Slack would render them.",
	Example: "  slk messages preview --text 'hello'\n  slk messages preview --blocks @blocks.json --metadata @metadata.json --human",
	RunE:    runMessagesPreview,
}

func init() {
	messagesCmd.AddCommand(messagesPreviewCmd)
	messagesPreviewCmd.Flags().StringP("mrkdwn", "m", "", "Slack mrkdwn text, inline, @file, or - for stdin")
	messagesPreviewCmd.Flags().StringP("text", "t", "", "Plain text, inline, @file, or - for stdin")
	messagesPreviewCmd.Flags().String("blocks", "", "Block Kit JSON, @file, or - for stdin")
	messagesPreviewCmd.Flags().String("metadata", "", "Slack metadata JSON, @file, or - for stdin")
}

type messagePreviewResult struct {
	OK       bool                    `json:"ok"`
	Text     string                  `json:"text,omitempty"`
	Mrkdwn   bool                    `json:"mrkdwn"`
	Blocks   []json.RawMessage       `json:"blocks,omitempty"`
	Metadata *slackapi.SlackMetadata `json:"metadata,omitempty"`
}

func (r *messagePreviewResult) Lines() []string {
	lines := []string{"Message payload is structurally valid"}
	if r.Text != "" {
		mode := "text"
		if r.Mrkdwn {
			mode = "mrkdwn"
		}
		lines = append(lines, "Text ("+mode+"): "+r.Text)
	}
	if len(r.Blocks) > 0 {
		lines = append(lines, fmt.Sprintf("Blocks: %d (validated; Slack rendering is not performed locally)", len(r.Blocks)))
	}
	if r.Metadata != nil {
		lines = append(lines, "Metadata event type: "+r.Metadata.EventType)
	}
	return lines
}

func runMessagesPreview(cmd *cobra.Command, _ []string) error {
	mrkdwn, _ := cmd.Flags().GetString("mrkdwn")
	plain, _ := cmd.Flags().GetString("text")
	blocksValue, _ := cmd.Flags().GetString("blocks")
	metadataValue, _ := cmd.Flags().GetString("metadata")
	stdinCount := 0
	for _, value := range []string{mrkdwn, plain, blocksValue, metadataValue} {
		if value == "-" {
			stdinCount++
		}
	}
	if stdinCount > 1 {
		return fmt.Errorf("only one input may read stdin")
	}
	var err error
	if mrkdwn != "" {
		mrkdwn, err = readLocalTextArgument(cmd, mrkdwn)
		if err != nil {
			return err
		}
	}
	if plain != "" {
		plain, err = readLocalTextArgument(cmd, plain)
		if err != nil {
			return err
		}
	}
	if mrkdwn != "" && plain != "" {
		return fmt.Errorf("choose only one of --mrkdwn or --text")
	}
	blocks, err := parseBlocksJSONInput(cmd, blocksValue)
	if err != nil {
		return err
	}
	metadata, err := parseMessageMetadataJSONInput(cmd, metadataValue)
	if err != nil {
		return err
	}
	if mrkdwn == "" && plain == "" && len(blocks) == 0 && metadata == nil {
		return fmt.Errorf("provide --mrkdwn, --text, --blocks, or --metadata")
	}
	result := &messagePreviewResult{OK: true, Text: plain, Mrkdwn: mrkdwn != ""}
	if mrkdwn != "" {
		result.Text = mrkdwn
	}
	for _, block := range blocks {
		raw, err := json.Marshal(block)
		if err != nil {
			return fmt.Errorf("encode block: %w", err)
		}
		result.Blocks = append(result.Blocks, raw)
	}
	result.Metadata = metadata
	return output.Print(cmd, result)
}

func readLocalTextArgument(cmd *cobra.Command, value string) (string, error) {
	switch {
	case value == "-":
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(data) == 0 {
			return "", fmt.Errorf("stdin input is empty")
		}
		return string(data), nil
	case strings.HasPrefix(value, "@"):
		path := strings.TrimSpace(strings.TrimPrefix(value, "@"))
		if path == "" {
			return "", fmt.Errorf("text file path is required after @")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read text file %s: %w", path, err)
		}
		return string(data), nil
	default:
		return value, nil
	}
}

func parseBlocksJSONInput(cmd *cobra.Command, value string) ([]slackapi.Block, error) {
	if value == "" {
		return nil, nil
	}
	resolved, err := readLocalJSONArgument(cmd, value)
	if err != nil {
		return nil, err
	}
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal([]byte(resolved), &rawBlocks); err != nil {
		return nil, fmt.Errorf("invalid blocks JSON array: %w", err)
	}
	if rawBlocks == nil {
		return nil, fmt.Errorf("blocks must be a JSON array, not null")
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

func parseAttachmentsJSON(value string) ([]slackapi.Attachment, error) {
	if value == "" {
		return nil, nil
	}
	resolved, err := readJSONArgument(value)
	if err != nil {
		return nil, fmt.Errorf("read attachments: %w", err)
	}
	var attachments []slackapi.Attachment
	if err := json.Unmarshal([]byte(resolved), &attachments); err != nil {
		return nil, fmt.Errorf("invalid attachments JSON array: %w", err)
	}
	return attachments, nil
}

func parseAttachmentsJSONInput(cmd *cobra.Command, value string) ([]slackapi.Attachment, error) {
	if value == "" {
		return nil, nil
	}
	resolved, err := readLocalJSONArgument(cmd, value)
	if err != nil {
		return nil, fmt.Errorf("read attachments: %w", err)
	}
	var attachments []slackapi.Attachment
	if err := json.Unmarshal([]byte(resolved), &attachments); err != nil {
		return nil, fmt.Errorf("invalid attachments JSON array: %w", err)
	}
	if attachments == nil {
		return nil, fmt.Errorf("attachments must be a JSON array, not null")
	}
	return attachments, nil
}

func parseMessageMetadataJSON(value string) (*slackapi.SlackMetadata, error) {
	if value == "" {
		return nil, nil
	}
	resolved, err := readJSONArgument(value)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	return decodeMessageMetadata(resolved)
}

func parseMessageMetadataJSONInput(cmd *cobra.Command, value string) (*slackapi.SlackMetadata, error) {
	if value == "" {
		return nil, nil
	}
	resolved, err := readLocalJSONArgument(cmd, value)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	return decodeMessageMetadata(resolved)
}

func parseMessageMetadataEditInput(cmd *cobra.Command, value string) (*slackapi.SlackMetadata, bool, error) {
	if value == "" {
		return nil, false, nil
	}
	resolved, err := readLocalJSONArgument(cmd, value)
	if err != nil {
		return nil, false, fmt.Errorf("read metadata: %w", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resolved), &object); err != nil || object == nil {
		return nil, false, fmt.Errorf("invalid metadata JSON object")
	}
	if len(object) == 0 {
		return nil, true, nil
	}
	metadata, err := decodeMessageMetadata(resolved)
	return metadata, false, err
}

func decodeMessageMetadata(value string) (*slackapi.SlackMetadata, error) {
	var metadata slackapi.SlackMetadata
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("invalid metadata JSON object: %w", err)
	}
	if strings.TrimSpace(metadata.EventType) == "" {
		return nil, fmt.Errorf("metadata event_type is required")
	}
	if metadata.EventPayload == nil {
		metadata.EventPayload = map[string]any{}
	}
	return &metadata, nil
}

func readLocalJSONArgument(cmd *cobra.Command, value string) (string, error) {
	switch {
	case value == "-":
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return "", fmt.Errorf("stdin JSON is empty")
		}
		return string(data), nil
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
