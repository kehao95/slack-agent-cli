package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/channels"
	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/output"
	"github.com/contentsquare/slack-cli/internal/slack"
	"github.com/spf13/cobra"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Channel operations",
	Long:  "List, inspect, and manage Slack channels accessible to the bot.",
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List channels",
	Long:  "List public and private channels the bot has access to via conversations.list.",
	RunE:  runChannelsList,
}

func init() {
	rootCmd.AddCommand(channelsCmd)
	channelsCmd.AddCommand(channelsListCmd)

	channelsListCmd.Flags().Bool("include-archived", false, "Include archived channels")
	channelsListCmd.Flags().Int("limit", 200, "Maximum channels per page")
	channelsListCmd.Flags().String("cursor", "", "Continuation cursor")
	channelsListCmd.Flags().StringSlice("types", []string{"public_channel", "private_channel"}, "Conversation types to include")
	channelsListCmd.Flags().Bool("refresh-cache", false, "Force refresh of cached channel metadata")
}

func runChannelsList(cmd *cobra.Command, args []string) error {
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
	service := channels.NewService(client)

	// Create resolver to manage cache for channel metadata
	resolver := channels.NewCachedResolver(client, cacheStore)

	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	types, _ := cmd.Flags().GetStringSlice("types")
	refreshCache, _ := cmd.Flags().GetBool("refresh-cache")

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Handle cache refresh - this will also pre-populate the cache
	if refreshCache {
		if err := resolver.RefreshCache(ctx); err != nil {
			return fmt.Errorf("refresh cache: %w", err)
		}
	}

	result, err := service.List(ctx, channels.ListParams{
		Limit:           limit,
		Cursor:          cursor,
		IncludeArchived: includeArchived,
		Types:           types,
	})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}
