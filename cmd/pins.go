package cmd

import (
	"fmt"

	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
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
  slack-agent-cli pins add --channel "#general" --ts "1705312365.000100"

  # Pin with human-readable output
  slack-agent-cli pins add --channel "#general" --ts "1705312365.000100" --human`,
	RunE: runPinsAdd,
}

var pinsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Unpin a message",
	Long:  "Remove a pinned message from a Slack channel.",
	Example: `  # Unpin a message
  slack-agent-cli pins remove --channel "#general" --ts "1705312365.000100"

  # Unpin with human-readable output
  slack-agent-cli pins remove --channel "#general" --ts "1705312365.000100" --human`,
	RunE: runPinsRemove,
}

var pinsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pinned messages",
	Long:  "List all pinned messages in a Slack channel.",
	Example: `  # List pinned messages
  slack-agent-cli pins list --channel "#general"

  # List with human-readable output
  slack-agent-cli pins list --channel "#general" --human`,
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

	// Add the pin
	if err := cmdCtx.Client.AddPin(cmdCtx.Ctx, channelID, timestamp); err != nil {
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

	// Remove the pin
	if err := cmdCtx.Client.RemovePin(cmdCtx.Ctx, channelID, timestamp); err != nil {
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
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	channelInput, _ := cmd.Flags().GetString("channel")

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// List pins
	result, err := cmdCtx.Client.ListPins(cmdCtx.Ctx, channelID)
	if err != nil {
		return fmt.Errorf("list pins: %w", err)
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}
