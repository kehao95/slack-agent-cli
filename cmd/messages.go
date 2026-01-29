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

func init() {
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.AddCommand(messagesListCmd)

	messagesListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	messagesListCmd.Flags().IntP("limit", "l", 50, "Maximum messages to return")
	messagesListCmd.Flags().String("since", "", "Messages after this time (ISO or relative like 1h)")
	messagesListCmd.Flags().String("until", "", "Messages before this time")
	messagesListCmd.Flags().String("thread", "", "Thread timestamp to fetch replies")
	messagesListCmd.Flags().Bool("refresh-cache", false, "Force refresh of cached channel/user metadata")
	messagesListCmd.MarkFlagRequired("channel")
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
	resolver := channels.NewCachedResolver(client, cacheStore)

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
		if err := resolver.RefreshCache(ctx); err != nil {
			return fmt.Errorf("refresh cache: %w", err)
		}
	}

	channelID, err := resolver.ResolveID(ctx, channelInput)
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
	result.Channel = channelInput
	return output.Print(cmd, result)
}
