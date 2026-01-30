package cmd

import (
	"fmt"

	"github.com/kehao95/slack-agent-cli/internal/messages"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
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
	Long: `Fetch message history from a Slack channel using conversations.history API.

Output (JSON):
  {
    "channel": "channel_id",
    "channel_name": "#general",
    "messages": [
      {
        "type": "message",
        "user": "U123ABC",
        "text": "message text",
        "ts": "1705312365.000100",
        "thread_ts": "1705312365.000100",  // Present if in thread
        "edited": {"user": "U123ABC", "ts": "..."},  // Present if edited
        "reactions": [{"name": "thumbsup", "count": 2, "users": ["U123"]}],
        "reply_count": 5  // Number of replies in thread
      }
    ],
    "has_more": true,
    "next_cursor": "bmV4dF90czox..."
  }

Channel Resolution:
  - Channel IDs (C123ABC) work directly without cache lookup
  - Channel names (#general) use cache, fallback to API if not found
  - Use 'cache populate channels' to pre-warm cache and avoid API calls`,
	Example: `  # Get last 20 messages
  slk messages list --channel "#general" --limit 20

  # Get messages from the last hour
  slk messages list --channel "#general" --since 1h

  # Get thread replies
  slk messages list --channel "#general" --thread "1705312365.000100"
  
  # Force refresh cached channel/user metadata
  slk messages list --channel "#general" --refresh-cache

  # Continue pagination with cursor
  slk messages list --channel "#general" --cursor "bmV4dF90czox..."`,
	RunE: runMessagesList,
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search messages",
	Long: `Search messages across the workspace.

Output (JSON):
  {
    "query": "search terms",
    "total": 42,
    "matches": [
      {
        "type": "message",
        "user": "U123ABC",
        "username": "alice",
        "text": "message text",
        "ts": "1705312365.000100",
        "channel": {"id": "C123", "name": "general"},
        "permalink": "https://workspace.slack.com/archives/..."
      }
    ],
    "pagination": {"total_count": 42, "page": 1, "page_count": 3}
  }

Search Syntax:
  - Basic: "error logs"
  - From user: "from:@alice deployment"
  - In channel: "in:#general bug"
  - Combined: "from:@alice in:#general error"`,
	Example: `  # Basic search
  slk messages search --query "deployment failed"

  # Search with advanced syntax
  slk messages search --query "from:@alice in:#general"

  # Search and sort by timestamp
  slk messages search --query "error" --sort timestamp --limit 20

  # Search and sort by relevance
  slk messages search --query "bug" --sort score`,
	RunE: runMessagesSearch,
}

var messagesSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message",
	Long: `Send a message to a channel or user.

Output (JSON):
  {
    "ok": true,
    "channel": "#general",
    "ts": "1705312365.000100",
    "message": {
      "type": "message",
      "user": "U123ABC",
      "text": "message text",
      "ts": "1705312365.000100"
    }
  }

Text Input:
  - Use --text flag for simple messages
  - Pipe from stdin for multi-line or dynamic content
  - Use --blocks for rich formatting (Block Kit JSON)`,
	Example: `  # Simple message
  slk messages send --channel "#general" --text "Hello from CLI!"

  # Reply in thread
  slk messages send --channel "#general" --thread "1705312365.000100" --text "Thread reply"

  # Pipe message content
  echo "Multi-line\nmessage" | slk messages send --channel "#general"

  # Send to user DM
  slk messages send --channel "@alice" --text "Private message"`,
	RunE: runMessagesSend,
}

var messagesEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit a message",
	Long: `Edit an existing message sent by you.

Output (JSON):
  {
    "ok": true,
    "channel": "#general",
    "ts": "1705312365.000100",
    "text": "updated text"
  }

Timestamp Format:
  Slack message timestamps are in format "1705312365.000100"
  - Obtain from 'messages list' output or message permalink
  - Copy from the 'ts' field in JSON output`,
	Example: `  # Edit a message
  slk messages edit --channel "#general" --ts "1705312365.000100" --text "Updated text"

  # Edit with JSON output
  slk messages edit --channel "#general" --ts "1705312365.000100" --text "New message"`,
	RunE: runMessagesEdit,
}

var messagesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a message",
	Long: `Delete a message sent by you.

Output (JSON):
  {
    "ok": true,
    "channel": "#general",
    "ts": "1705312365.000100"
  }

Timestamp Format:
  Slack message timestamps are in format "1705312365.000100"
  - Obtain from 'messages list' output or message permalink
  - Copy from the 'ts' field in JSON output`,
	Example: `  # Delete a message
  slk messages delete --channel "#general" --ts "1705312365.000100"

  # Delete with JSON output
  slk messages delete --channel "#general" --ts "1705312365.000100"`,
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
		stdinText, err := readStdinIfPiped()
		if err != nil {
			return err
		}
		text = stdinText
	}

	// Parse blocks if provided
	blocks, err := parseBlocksJSON(blocksJSON)
	if err != nil {
		return err
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
