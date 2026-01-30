package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kehao95/slack-agent-cli/internal/messages"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Message operations",
	Long:  "Send, list, edit, and delete Slack messages.",
}

var messagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages from a channel",
	Long:  "Fetch message history from a Slack channel using conversations.history API.",
	Example: `  # Get last 20 messages
  slack-agent-cli messages list --channel "#general" --limit 20

  # Get messages from the last hour
  slack-agent-cli messages list --channel "#general" --since 1h

  # Get thread replies
  slack-agent-cli messages list --channel "#general" --thread "1705312365.000100"
  
  # Force refresh cached channel/user metadata
  slack-agent-cli messages list --channel "#general" --refresh-cache`,
	RunE: runMessagesList,
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search messages",
	Long:  "Search messages across the workspace.",
	Example: `  # Basic search
  slack-agent-cli messages search --query "deployment failed"

  # Search with advanced syntax
  slack-agent-cli messages search --query "from:@alice in:#general"

  # Search and sort by timestamp
  slack-agent-cli messages search --query "error" --sort timestamp --limit 20

  # Search with human-readable output
  slack-agent-cli messages search --query "bug" --human`,
	RunE: runMessagesSearch,
}

var messagesSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message",
	Long:  "Send a message to a channel or user.",
	Example: `  # Simple message
  slack-agent-cli messages send --channel "#general" --text "Hello from CLI!"

  # Reply in thread
  slack-agent-cli messages send --channel "#general" --thread "1705312365.000100" --text "Thread reply"

  # Pipe message content
  echo "Multi-line\nmessage" | slack-agent-cli messages send --channel "#general"

  # Send to user DM
  slack-agent-cli messages send --channel "@alice" --text "Private message"`,
	RunE: runMessagesSend,
}

var messagesEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit a message",
	Long:  "Edit an existing message sent by you.",
	Example: `  # Edit a message
  slack-agent-cli messages edit --channel "#general" --ts "1705312365.000100" --text "Updated text"

  # Edit with human-readable output
  slack-agent-cli messages edit --channel "#general" --ts "1705312365.000100" --text "New message" --human`,
	RunE: runMessagesEdit,
}

var messagesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a message",
	Long:  "Delete a message sent by you.",
	Example: `  # Delete a message
  slack-agent-cli messages delete --channel "#general" --ts "1705312365.000100"

  # Delete with human-readable output
  slack-agent-cli messages delete --channel "#general" --ts "1705312365.000100" --human`,
	RunE: runMessagesDelete,
}

func init() {
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSearchCmd)
	messagesCmd.AddCommand(messagesSendCmd)
	messagesCmd.AddCommand(messagesEditCmd)
	messagesCmd.AddCommand(messagesDeleteCmd)

	messagesListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	messagesListCmd.Flags().IntP("limit", "l", 50, "Maximum messages to return")
	messagesListCmd.Flags().String("since", "", "Messages after this time (ISO or relative like 1h)")
	messagesListCmd.Flags().String("until", "", "Messages before this time")
	messagesListCmd.Flags().String("thread", "", "Thread timestamp to fetch replies")
	messagesListCmd.Flags().Bool("refresh-cache", false, "Force refresh of cached channel/user metadata")
	messagesListCmd.MarkFlagRequired("channel")

	messagesSearchCmd.Flags().StringP("query", "q", "", "Search query (required)")
	messagesSearchCmd.Flags().IntP("limit", "l", 20, "Maximum results to return")
	messagesSearchCmd.Flags().String("sort", "timestamp", "Sort by 'score' or 'timestamp'")
	messagesSearchCmd.Flags().String("sort-dir", "desc", "Sort direction 'asc' or 'desc'")
	messagesSearchCmd.MarkFlagRequired("query")

	messagesSendCmd.Flags().StringP("channel", "c", "", "Target channel or @user (required)")
	messagesSendCmd.Flags().StringP("text", "t", "", "Message text")
	messagesSendCmd.Flags().String("thread", "", "Thread timestamp to reply in")
	messagesSendCmd.Flags().String("blocks", "", "Block Kit JSON")
	messagesSendCmd.Flags().Bool("unfurl-links", true, "Unfurl URLs in message")
	messagesSendCmd.Flags().Bool("unfurl-media", true, "Unfurl media in message")
	messagesSendCmd.MarkFlagRequired("channel")

	messagesEditCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	messagesEditCmd.Flags().String("ts", "", "Message timestamp (required)")
	messagesEditCmd.Flags().StringP("text", "t", "", "New message text (required)")
	messagesEditCmd.MarkFlagRequired("channel")
	messagesEditCmd.MarkFlagRequired("ts")
	messagesEditCmd.MarkFlagRequired("text")

	messagesDeleteCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	messagesDeleteCmd.Flags().String("ts", "", "Message timestamp (required)")
	messagesDeleteCmd.MarkFlagRequired("channel")
	messagesDeleteCmd.MarkFlagRequired("ts")
}

func runMessagesList(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	fetcher := slack.NewMessageFetcher(cmdCtx.Client)
	service := messages.NewService(fetcher)

	channelInput, _ := cmd.Flags().GetString("channel")
	limit, _ := cmd.Flags().GetInt("limit")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	thread, _ := cmd.Flags().GetString("thread")
	refreshCache, _ := cmd.Flags().GetBool("refresh-cache")

	// Handle cache refresh
	if refreshCache {
		if err := cmdCtx.ChannelResolver.RefreshCache(cmdCtx.Ctx); err != nil {
			return fmt.Errorf("refresh cache: %w", err)
		}
		if err := cmdCtx.UserResolver.RefreshCache(cmdCtx.Ctx); err != nil {
			return fmt.Errorf("refresh user cache: %w", err)
		}
		if err := cmdCtx.UserGroupResolver.RefreshCache(cmdCtx.Ctx); err != nil {
			return fmt.Errorf("refresh usergroup cache: %w", err)
		}
	}

	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}
	result, err := service.List(cmdCtx.Ctx, messages.Params{
		Channel: channelID,
		Limit:   limit,
		Since:   since,
		Until:   until,
		Thread:  thread,
	})
	if err != nil {
		return err
	}

	// Set display metadata for human-readable output
	result.Channel = channelInput
	result.ChannelName = channelInput
	result.SetUserResolver(cmdCtx.Ctx, cmdCtx.UserResolver)
	result.SetUserGroupResolver(cmdCtx.Ctx, cmdCtx.UserGroupResolver)

	return output.Print(cmd, result)
}

func runMessagesSearch(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	query, _ := cmd.Flags().GetString("query")
	limit, _ := cmd.Flags().GetInt("limit")
	sortBy, _ := cmd.Flags().GetString("sort")
	sortDir, _ := cmd.Flags().GetString("sort-dir")

	// Validate sort parameters
	if sortBy != "score" && sortBy != "timestamp" {
		return fmt.Errorf("invalid sort value '%s': must be 'score' or 'timestamp'", sortBy)
	}
	if sortDir != "asc" && sortDir != "desc" {
		return fmt.Errorf("invalid sort-dir value '%s': must be 'asc' or 'desc'", sortDir)
	}

	userClient := slack.NewUserClient(cmdCtx.Config.UserToken)
	result, err := userClient.SearchMessages(cmdCtx.Ctx, query, slack.SearchParams{
		Count:     limit,
		Page:      1,
		SortBy:    sortBy,
		SortDir:   sortDir,
		Highlight: false,
	})
	if err != nil {
		return fmt.Errorf("search messages: %w", err)
	}

	return output.Print(cmd, result)
}

func runMessagesSend(cmd *cobra.Command, args []string) error {
	channelInput, _ := cmd.Flags().GetString("channel")
	text, _ := cmd.Flags().GetString("text")
	thread, _ := cmd.Flags().GetString("thread")
	blocksJSON, _ := cmd.Flags().GetString("blocks")
	unfurlLinks, _ := cmd.Flags().GetBool("unfurl-links")
	unfurlMedia, _ := cmd.Flags().GetBool("unfurl-media")

	// If no text flag, try reading from stdin
	if text == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			text = string(data)
		}
	}

	// Parse blocks if provided
	var blocks []slackapi.Block
	if blocksJSON != "" {
		// Parse Block Kit JSON
		var rawBlocks []json.RawMessage
		if err := json.Unmarshal([]byte(blocksJSON), &rawBlocks); err != nil {
			return fmt.Errorf("invalid blocks JSON: %w", err)
		}

		for _, raw := range rawBlocks {
			var blockType struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(raw, &blockType); err != nil {
				return fmt.Errorf("parse block type: %w", err)
			}

			// Unmarshal each block based on type
			var block slackapi.Block
			switch blockType.Type {
			case "section":
				var b slackapi.SectionBlock
				if err := json.Unmarshal(raw, &b); err != nil {
					return fmt.Errorf("parse section block: %w", err)
				}
				block = &b
			case "divider":
				var b slackapi.DividerBlock
				if err := json.Unmarshal(raw, &b); err != nil {
					return fmt.Errorf("parse divider block: %w", err)
				}
				block = &b
			case "header":
				var b slackapi.HeaderBlock
				if err := json.Unmarshal(raw, &b); err != nil {
					return fmt.Errorf("parse header block: %w", err)
				}
				block = &b
			default:
				return fmt.Errorf("unsupported block type: %s", blockType.Type)
			}
			blocks = append(blocks, block)
		}
	}

	// Validate we have either text or blocks
	if text == "" && len(blocks) == 0 {
		return fmt.Errorf("message text is required (use --text, --blocks, or pipe via stdin)")
	}

	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// Send the message
	result, err := cmdCtx.Client.PostMessage(cmdCtx.Ctx, channelID, slack.PostMessageOptions{
		Text:        text,
		ThreadTS:    thread,
		Blocks:      blocks,
		UnfurlLinks: unfurlLinks,
		UnfurlMedia: unfurlMedia,
	})
	if err != nil {
		return err
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}

func runMessagesEdit(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")
	text, _ := cmd.Flags().GetString("text")

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// Edit the message
	result, err := cmdCtx.Client.EditMessage(cmdCtx.Ctx, channelID, timestamp, text)
	if err != nil {
		return err
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}

func runMessagesDelete(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// Delete the message
	result, err := cmdCtx.Client.DeleteMessage(cmdCtx.Ctx, channelID, timestamp)
	if err != nil {
		return err
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}
