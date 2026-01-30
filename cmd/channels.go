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
	Long:  "List, inspect, and manage Slack channels accessible to the user.",
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List channels",
	Long:  "List public and private channels the user has access to via conversations.list.",
	RunE:  runChannelsList,
}

var channelsJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a channel",
	Long:  "Join a public Slack channel.",
	Example: `  # Join a channel by name
  slack-cli channels join --channel "#general"

  # Join a channel by ID
  slack-cli channels join --channel "CBMCT6HTN"`,
	RunE: runChannelsJoin,
}

var channelsLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave a channel",
	Long:  "Leave a Slack channel.",
	Example: `  # Leave a channel by name
  slack-cli channels leave --channel "#general"

  # Leave a channel by ID
  slack-cli channels leave --channel "CBMCT6HTN"`,
	RunE: runChannelsLeave,
}

func init() {
	rootCmd.AddCommand(channelsCmd)
	channelsCmd.AddCommand(channelsListCmd)
	channelsCmd.AddCommand(channelsJoinCmd)
	channelsCmd.AddCommand(channelsLeaveCmd)

	channelsListCmd.Flags().Bool("include-archived", false, "Include archived channels")
	channelsListCmd.Flags().Int("limit", 200, "Maximum channels per page")
	channelsListCmd.Flags().String("cursor", "", "Continuation cursor")
	channelsListCmd.Flags().StringSlice("types", []string{"public_channel"}, "Conversation types to include (public_channel requires channels:read, private_channel requires groups:read)")
	channelsListCmd.Flags().Bool("refresh-cache", false, "Force refresh of cached channel metadata")

	// Flags for join command
	channelsJoinCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	channelsJoinCmd.MarkFlagRequired("channel")

	// Flags for leave command
	channelsLeaveCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	channelsLeaveCmd.MarkFlagRequired("channel")
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

	client := slack.New(cfg.UserToken)
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

func runChannelsJoin(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	channelInput, _ := cmd.Flags().GetString("channel")

	// Initialize cache store
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	client := slack.New(cfg.UserToken)
	channelResolver := channels.NewCachedResolver(client, cacheStore)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Resolve channel name to ID
	channelID, err := channelResolver.ResolveID(ctx, channelInput)
	if err != nil {
		return err
	}

	// Join the channel
	result, err := client.JoinChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("join channel: %w", err)
	}

	// Use the original input for display
	result.Channel = channelInput

	return output.Print(cmd, result)
}

func runChannelsLeave(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	channelInput, _ := cmd.Flags().GetString("channel")

	// Initialize cache store
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	client := slack.New(cfg.UserToken)
	channelResolver := channels.NewCachedResolver(client, cacheStore)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Resolve channel name to ID
	channelID, err := channelResolver.ResolveID(ctx, channelInput)
	if err != nil {
		return err
	}

	// Leave the channel
	result, err := client.LeaveChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("leave channel: %w", err)
	}

	// Use the original input for display
	result.Channel = channelInput

	return output.Print(cmd, result)
}
