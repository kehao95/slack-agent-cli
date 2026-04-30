package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/eventstore"
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
    "channel": "#general",
    "channel_id": "C123ABC",
    "channel_name": "general",
    "messages": [
      {
        "type": "message",
        "user": "@alice",
        "user_id": "U123ABC",
        "username": "Alice Example",
        "text": "message text",
        "ts": "1705312365.000100",
        "thread_ts": "1705312365.000100",
        "edited": {"user": "@alice", "user_id": "U123ABC", "ts": "..."},
        "reactions": [{"name": "thumbsup", "count": 2, "users": ["@alice"], "user_ids": ["U123ABC"]}],
        "reply_count": 5  // Number of replies in thread
      }
    ],
    "has_more": true,
    "next_cursor": "bmV4dF90czox..."
  }

By default JSON resolves channel and user references for readability while preserving raw IDs in companion *_id fields. Use --raw-json to keep Slack IDs in their original fields.

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
    "messages": {
      "total": 42,
      "matches": [
        {
          "type": "message",
          "user": "@alice",
          "user_id": "U123ABC",
          "username": "alice",
          "text": "message text",
          "ts": "1705312365.000100",
          "channel": {"id": "C123", "name": "#general"},
          "permalink": "https://workspace.slack.com/archives/..."
        }
      ]
    }
  }

By default JSON resolves channel and user references for readability while preserving raw IDs in companion *_id fields. Use --raw-json to keep Slack IDs in their original fields.

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
  - Choose exactly one of --mrkdwn, --text, or --blocks
  - Use --mrkdwn for Slack-formatted message text
  - Use --text only for plain text when you intentionally do not want Slack formatting
  - Use --mrkdwn - or --text - to read that format from stdin
  - The CLI does not validate or convert formatting; Slack receives the text as-is
  - Slack mrkdwn is not GitHub/CommonMark Markdown
  - Slack mrkdwn examples: *bold*, _italic_, ~strike~, inline code with backticks, triple-backtick code blocks, <https://example.com|link text>, <@USERID>
  - Slack top-level message text has no real bullet-list syntax; mimic lists with plain lines like "- item"
  - Use --blocks for true rich lists, headings, or more structured layouts
  - Slack message text does not support Markdown headings or tables`,
	Example: `  # Simple message
  slk messages send --channel "#general" --mrkdwn "Hello from CLI!"

  # Slack mrkdwn formatting
  slk messages send --channel "#general" --mrkdwn "*Done:* see <https://example.com|docs>"

  # Reply in thread
  slk messages send --channel "#general" --thread "1705312365.000100" --mrkdwn "Thread reply"

  # Read Slack mrkdwn from stdin
  printf '*Done:* see <https://example.com|docs>\n' | slk messages send --channel "#general" --mrkdwn -

  # Mimic a list in Slack mrkdwn
  printf '*Plan:*\n- claim root messages\n- route thread replies\n' | slk messages send --channel "#general" --mrkdwn -

  # Send to user DM
  slk messages send --channel "@alice" --mrkdwn "Private message"`,
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

var messagesNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Wait for the next cached message event",
	Long: `Wait for the next message-shaped event in the local daemon cache.

This is the agent-friendly blocking primitive for interactive loops. Run "slk daemon run" in a supervisor first.`,
	Example: `  # Wait for the next new message in a channel
  slk messages next --channel "#_bot-testing" --since latest --timeout 60s

  # Wait for the next reply in a thread, ignoring the bot's own messages
  slk messages next --channel "#_bot-testing" --thread "$THREAD_TS" --since "$CURSOR" --exclude-self --timeout 5m`,
	RunE: runMessagesNext,
}

func init() {
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSearchCmd)
	messagesCmd.AddCommand(messagesSendCmd)
	messagesCmd.AddCommand(messagesEditCmd)
	messagesCmd.AddCommand(messagesDeleteCmd)
	messagesCmd.AddCommand(messagesNextCmd)

	messagesListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	messagesListCmd.Flags().IntP("limit", "l", 50, "Maximum messages to return")
	messagesListCmd.Flags().String("since", "", "Messages after this time (ISO or relative like 1h)")
	messagesListCmd.Flags().String("until", "", "Messages before this time")
	messagesListCmd.Flags().String("thread", "", "Thread timestamp to fetch replies")
	messagesListCmd.Flags().Bool("refresh-cache", false, "Force refresh of cached channel/user metadata")
	messagesListCmd.Flags().Bool("resolved-json", true, "Resolve channel and user references in JSON output")
	messagesListCmd.Flags().Bool("raw-json", false, "Preserve raw Slack IDs in JSON output")
	messagesListCmd.MarkFlagRequired("channel")

	messagesSearchCmd.Flags().StringP("query", "q", "", "Search query (required)")
	messagesSearchCmd.Flags().IntP("limit", "l", 20, "Maximum results to return")
	messagesSearchCmd.Flags().String("sort", "timestamp", "Sort by 'score' or 'timestamp'")
	messagesSearchCmd.Flags().String("sort-dir", "desc", "Sort direction 'asc' or 'desc'")
	messagesSearchCmd.Flags().Bool("resolved-json", true, "Resolve channel and user references in JSON output")
	messagesSearchCmd.Flags().Bool("raw-json", false, "Preserve raw Slack IDs in JSON output")
	messagesSearchCmd.MarkFlagRequired("query")

	messagesSendCmd.Flags().StringP("channel", "c", "", "Target channel or @user (required)")
	messagesSendCmd.Flags().StringP("mrkdwn", "m", "", "Slack mrkdwn message text (sent as-is)")
	messagesSendCmd.Flags().StringP("text", "t", "", "Plain message text (sent as-is; no Slack formatting intent)")
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

	messagesNextCmd.Flags().StringP("channel", "c", "", "Channel name or ID")
	messagesNextCmd.Flags().String("thread", "", "Thread timestamp to wait in")
	messagesNextCmd.Flags().String("user", "", "Restrict to a Slack user ID")
	messagesNextCmd.Flags().String("since", "", "Start after local cursor, or from duration/RFC3339 time (default: latest)")
	messagesNextCmd.Flags().Bool("exclude-self", false, "Exclude messages produced by the active auth identity")
	messagesNextCmd.Flags().Duration("timeout", 0, "Maximum time to wait for a matching message (0 waits forever)")
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
	rawJSON, _ := cmd.Flags().GetBool("raw-json")
	resolvedJSON, _ := cmd.Flags().GetBool("resolved-json")

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

	// Set display metadata
	result.Channel = channelID
	// Resolve channel name for both JSON and human-readable output
	channelName := cmdCtx.ChannelResolver.ResolveName(cmdCtx.Ctx, channelID)
	if channelName != "" && channelName != channelID {
		result.ChannelName = strings.TrimPrefix(channelName, "#")
	} else {
		result.ChannelName = strings.TrimPrefix(channelInput, "#")
	}
	result.SetUserResolver(cmdCtx.Ctx, cmdCtx.UserResolver)
	result.SetUserGroupResolver(cmdCtx.Ctx, cmdCtx.UserGroupResolver)
	result.SetRawJSON(rawJSON || !resolvedJSON)

	return output.Print(cmd, result)
}

// isChannelID checks if a string looks like a channel ID (starts with C, D, or G followed by alphanumerics)
func isChannelID(s string) bool {
	if len(s) < 2 {
		return false
	}
	first := s[0]
	return (first == 'C' || first == 'D' || first == 'G') && strings.ToUpper(s) == s
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
	rawJSON, _ := cmd.Flags().GetBool("raw-json")
	resolvedJSON, _ := cmd.Flags().GetBool("resolved-json")

	// Validate sort parameters
	if sortBy != "score" && sortBy != "timestamp" {
		return fmt.Errorf("invalid sort value '%s': must be 'score' or 'timestamp'", sortBy)
	}
	if sortDir != "asc" && sortDir != "desc" {
		return fmt.Errorf("invalid sort-dir value '%s': must be 'asc' or 'desc'", sortDir)
	}

	userClient := slack.NewUserClient(cmdCtx.AuthToken)
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
	result.SetUserResolver(cmdCtx.Ctx, cmdCtx.UserResolver)
	result.SetChannelResolver(cmdCtx.Ctx, cmdCtx.ChannelResolver)
	result.SetRawJSON(rawJSON || !resolvedJSON)

	return output.Print(cmd, result)
}

func runMessagesSend(cmd *cobra.Command, args []string) error {
	channelInput, _ := cmd.Flags().GetString("channel")
	text, _ := cmd.Flags().GetString("text")
	mrkdwn, _ := cmd.Flags().GetString("mrkdwn")
	thread, _ := cmd.Flags().GetString("thread")
	blocksJSON, _ := cmd.Flags().GetString("blocks")
	unfurlLinks, _ := cmd.Flags().GetBool("unfurl-links")
	unfurlMedia, _ := cmd.Flags().GetBool("unfurl-media")

	// Parse blocks if provided
	blocks, err := parseBlocksJSON(blocksJSON)
	if err != nil {
		return err
	}

	if mrkdwn == "-" {
		mrkdwn, err = readRequiredStdin("mrkdwn")
		if err != nil {
			return err
		}
	}
	if text == "-" {
		text, err = readRequiredStdin("text")
		if err != nil {
			return err
		}
	}
	inputCount := 0
	if mrkdwn != "" {
		inputCount++
		text = mrkdwn
	}
	if text != "" && mrkdwn == "" {
		inputCount++
	}
	if len(blocks) > 0 {
		inputCount++
	}
	if inputCount != 1 {
		return fmt.Errorf("choose exactly one message input: --mrkdwn, --text, or --blocks")
	}

	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	if cmdCtx.AuthRole == config.RoleUser {
		if err := ensureUserRoleCanPostWithoutBotAttribution(cmdCtx); err != nil {
			return err
		}
	}

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
		AsUser:      cmdCtx.AuthRole == config.RoleUser,
	})
	if err != nil {
		return err
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}

func ensureUserRoleCanPostWithoutBotAttribution(cmdCtx *CommandContext) error {
	if strings.HasPrefix(strings.TrimSpace(cmdCtx.AuthToken), "xoxc-") {
		return nil
	}

	scopes, err := fetchTokenScopes(cmdCtx.Ctx, cmdCtx.AuthToken, cmdCtx.AuthCookie)
	if err != nil {
		return fmt.Errorf("verify user token scopes before sending: %w", err)
	}
	if containsCSVScope(scopes, "chat:write:user") {
		return nil
	}

	return fmt.Errorf(
		"role=user message sending requires Slack user scope chat:write:user to avoid app/bot attribution; current token scopes are %q. Reauthorize the Slack app with chat:write:user and run slk auth login with the new xoxp token",
		scopes,
	)
}

func fetchTokenScopes(ctx context.Context, token, cookie string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/auth.test", nil)
	if err != nil {
		return "", fmt.Errorf("create auth.test request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if cookie != "" {
		req.Header.Set("Cookie", "d="+cookie)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call auth.test: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("auth.test returned HTTP %d", resp.StatusCode)
	}
	return resp.Header.Get("x-oauth-scopes"), nil
}

func containsCSVScope(scopes string, want string) bool {
	for _, scope := range strings.Split(scopes, ",") {
		if strings.TrimSpace(scope) == want {
			return true
		}
	}
	return false
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

func runMessagesNext(cmd *cobra.Command, args []string) error {
	cmdCtx, _, store, err := openEventQueryStore(cmd, true)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	defer store.Close()

	filter, err := buildEventQueryFilter(cmd, cmdCtx, store, true)
	if err != nil {
		return err
	}
	filter.Type = "message"
	filter.Limit = 1

	timeout, _ := cmd.Flags().GetDuration("timeout")
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, err := store.Query(cmdCtx.Ctx, filter)
		if err != nil {
			return err
		}
		if len(events) > 0 {
			return printCachedMessageEvent(cmd, events[0])
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			cmd.SilenceUsage = true
			return cerrors.TimeoutError("timed out waiting for matching message")
		}
		select {
		case <-cmdCtx.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func printCachedMessageEvent(cmd *cobra.Command, event eventstore.Event) error {
	human, _ := cmd.Flags().GetBool("human")
	if human {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), formatHumanStreamEvent(streamEventFromStore(event)))
		return err
	}
	message := map[string]interface{}{
		"cursor":            event.Cursor,
		"received_at":       event.ReceivedAt,
		"type":              event.Type,
		"subtype":           event.Subtype,
		"channel":           event.Channel,
		"channel_id":        event.ChannelID,
		"conversation_type": event.ConversationType,
		"user":              event.User,
		"user_id":           event.UserID,
		"bot_id":            event.BotID,
		"ts":                event.TS,
		"thread_ts":         event.ThreadTS,
		"text":              event.Text,
		"is_thread_reply":   event.IsThreadReply,
		"is_thread_root":    event.IsThreadRoot,
		"is_self":           event.IsSelf,
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(message)
}
