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

var pinsCmd = &cobra.Command{
	Use:   "pins",
	Short: "Pin operations",
	Long:  "Pin, unpin, and list pinned messages in channels.",
}

var pinsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Pin a message",
	Long:  "Pin a message to a Slack channel.",
	Example: `  # Pin a message
  slack-cli pins add --channel "#general" --ts "1705312365.000100"

  # Pin with JSON output
  slack-cli pins add --channel "#general" --ts "1705312365.000100" --json`,
	RunE: runPinsAdd,
}

var pinsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Unpin a message",
	Long:  "Remove a pinned message from a Slack channel.",
	Example: `  # Unpin a message
  slack-cli pins remove --channel "#general" --ts "1705312365.000100"

  # Unpin with JSON output
  slack-cli pins remove --channel "#general" --ts "1705312365.000100" --json`,
	RunE: runPinsRemove,
}

var pinsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pinned messages",
	Long:  "List all pinned messages in a Slack channel.",
	Example: `  # List pinned messages
  slack-cli pins list --channel "#general"

  # List with JSON output
  slack-cli pins list --channel "#general" --json`,
	RunE: runPinsList,
}

func init() {
	rootCmd.AddCommand(pinsCmd)
	pinsCmd.AddCommand(pinsAddCmd)
	pinsCmd.AddCommand(pinsRemoveCmd)
	pinsCmd.AddCommand(pinsListCmd)

	// Flags for add command
	pinsAddCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	pinsAddCmd.Flags().String("ts", "", "Message timestamp (required)")
	pinsAddCmd.MarkFlagRequired("channel")
	pinsAddCmd.MarkFlagRequired("ts")

	// Flags for remove command
	pinsRemoveCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	pinsRemoveCmd.Flags().String("ts", "", "Message timestamp (required)")
	pinsRemoveCmd.MarkFlagRequired("channel")
	pinsRemoveCmd.MarkFlagRequired("ts")

	// Flags for list command
	pinsListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	pinsListCmd.MarkFlagRequired("channel")
}

func runPinsAdd(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")

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

	// Add the pin
	if err := client.AddPin(ctx, channelID, timestamp); err != nil {
		return fmt.Errorf("add pin: %w", err)
	}

	result := &slack.PinResult{
		OK:        true,
		Action:    "add",
		Channel:   channelInput,
		ChannelID: channelID,
		Timestamp: timestamp,
	}

	return output.Print(cmd, result)
}

func runPinsRemove(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")

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

	// Remove the pin
	if err := client.RemovePin(ctx, channelID, timestamp); err != nil {
		return fmt.Errorf("remove pin: %w", err)
	}

	result := &slack.PinResult{
		OK:        true,
		Action:    "remove",
		Channel:   channelInput,
		ChannelID: channelID,
		Timestamp: timestamp,
	}

	return output.Print(cmd, result)
}

func runPinsList(cmd *cobra.Command, args []string) error {
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

	// List pins
	result, err := client.ListPins(ctx, channelID)
	if err != nil {
		return fmt.Errorf("list pins: %w", err)
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}
