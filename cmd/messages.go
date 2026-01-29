package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/channels"
	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/messages"
	"github.com/contentsquare/slack-cli/internal/output"
	"github.com/contentsquare/slack-cli/internal/slack"
	"github.com/contentsquare/slack-cli/internal/usergroups"
	"github.com/contentsquare/slack-cli/internal/users"
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
  slack-cli messages list --channel "#general" --limit 20

  # Get messages from the last hour
  slack-cli messages list --channel "#general" --since 1h --json

  # Get thread replies
  slack-cli messages list --channel "#general" --thread "1705312365.000100"
  
  # Force refresh cached channel/user metadata
  slack-cli messages list --channel "#general" --refresh-cache`,
	RunE: runMessagesList,
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search messages",
	Long:  "Search messages across the workspace (requires user token).",
	Example: `  # Basic search
  slack-cli messages search --query "deployment failed"

  # Search with advanced syntax
  slack-cli messages search --query "from:@alice in:#general"

  # Search and sort by timestamp
  slack-cli messages search --query "error" --sort timestamp --limit 20

  # Search and get JSON output
  slack-cli messages search --query "bug" --json`,
	RunE: runMessagesSearch,
}

func init() {
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSearchCmd)

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
}

func runMessagesList(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	// Initialize cache store
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	client := slack.New(cfg.BotToken)
	fetcher := slack.NewMessageFetcher(client)
	service := messages.NewService(fetcher)
	channelResolver := channels.NewCachedResolver(client, cacheStore)
	userResolver := users.NewCachedResolver(client, cacheStore)
	userGroupResolver := usergroups.NewCachedResolver(client, cacheStore)

	channelInput, _ := cmd.Flags().GetString("channel")
	limit, _ := cmd.Flags().GetInt("limit")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	thread, _ := cmd.Flags().GetString("thread")
	refreshCache, _ := cmd.Flags().GetBool("refresh-cache")

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Handle cache refresh
	if refreshCache {
		if err := channelResolver.RefreshCache(ctx); err != nil {
			return fmt.Errorf("refresh cache: %w", err)
		}
		if err := userResolver.RefreshCache(ctx); err != nil {
			return fmt.Errorf("refresh user cache: %w", err)
		}
		if err := userGroupResolver.RefreshCache(ctx); err != nil {
			return fmt.Errorf("refresh usergroup cache: %w", err)
		}
	}

	channelID, err := channelResolver.ResolveID(ctx, channelInput)
	if err != nil {
		return err
	}
	result, err := service.List(ctx, messages.Params{
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
	result.SetUserResolver(ctx, userResolver)
	result.SetUserGroupResolver(ctx, userGroupResolver)

	return output.Print(cmd, result)
}

func runMessagesSearch(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	// Check for user token
	if cfg.UserToken == "" {
		return fmt.Errorf("messages search requires user token (set SLACK_USER_TOKEN or add user_token to config)")
	}

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

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	userClient := slack.NewUserClient(cfg.UserToken)
	result, err := userClient.SearchMessages(ctx, query, slack.SearchParams{
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
