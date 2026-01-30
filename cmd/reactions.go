package cmd

import (
	"fmt"

	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var reactionsCmd = &cobra.Command{
	Use:   "reactions",
	Short: "Reaction operations",
	Long:  "Add, remove, and list emoji reactions on messages.",
}

var reactionsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add reaction to message",
	Long:  "Add an emoji reaction to a Slack message.",
	Example: `  # Add thumbsup reaction
  slack-agent-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

  # Add custom emoji
  slack-agent-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "custom_emoji"`,
	RunE: runReactionsAdd,
}

var reactionsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove reaction from message",
	Long:  "Remove an emoji reaction from a Slack message.",
	Example: `  # Remove thumbsup reaction
  slack-agent-cli reactions remove --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

  # Remove custom emoji
  slack-agent-cli reactions remove --channel "#general" --ts "1705312365.000100" --emoji "custom_emoji"`,
	RunE: runReactionsRemove,
}

var reactionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List reactions on a message",
	Long:  "List all emoji reactions on a Slack message.",
	Example: `  # List reactions on a message
  slack-agent-cli reactions list --channel "#general" --ts "1705312365.000100"

  # List with human-readable output
  slack-agent-cli reactions list --channel "#general" --ts "1705312365.000100" --human`,
	RunE: runReactionsList,
}

func init() {
	rootCmd.AddCommand(reactionsCmd)
	reactionsCmd.AddCommand(reactionsAddCmd)
	reactionsCmd.AddCommand(reactionsRemoveCmd)
	reactionsCmd.AddCommand(reactionsListCmd)

	// Flags for add command
	reactionsAddCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	reactionsAddCmd.Flags().String("ts", "", "Message timestamp (required)")
	reactionsAddCmd.Flags().StringP("emoji", "e", "", "Emoji name without colons (required)")
	reactionsAddCmd.MarkFlagRequired("channel")
	reactionsAddCmd.MarkFlagRequired("ts")
	reactionsAddCmd.MarkFlagRequired("emoji")

	// Flags for remove command
	reactionsRemoveCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	reactionsRemoveCmd.Flags().String("ts", "", "Message timestamp (required)")
	reactionsRemoveCmd.Flags().StringP("emoji", "e", "", "Emoji name without colons (required)")
	reactionsRemoveCmd.MarkFlagRequired("channel")
	reactionsRemoveCmd.MarkFlagRequired("ts")
	reactionsRemoveCmd.MarkFlagRequired("emoji")

	// Flags for list command
	reactionsListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	reactionsListCmd.Flags().String("ts", "", "Message timestamp (required)")
	reactionsListCmd.MarkFlagRequired("channel")
	reactionsListCmd.MarkFlagRequired("ts")
}

func runReactionsAdd(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")
	emoji, _ := cmd.Flags().GetString("emoji")

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// Add the reaction
	if err := cmdCtx.Client.AddReaction(cmdCtx.Ctx, channelID, timestamp, emoji); err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}

	result := &slack.ReactionResult{
		OK:        true,
		Action:    "add",
		Channel:   channelInput,
		ChannelID: channelID,
		Timestamp: timestamp,
		Emoji:     emoji,
	}

	return output.Print(cmd, result)
}

func runReactionsRemove(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	channelInput, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")
	emoji, _ := cmd.Flags().GetString("emoji")

	// Resolve channel name to ID
	channelID, err := cmdCtx.ResolveChannel(channelInput)
	if err != nil {
		return err
	}

	// Remove the reaction
	if err := cmdCtx.Client.RemoveReaction(cmdCtx.Ctx, channelID, timestamp, emoji); err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}

	result := &slack.ReactionResult{
		OK:        true,
		Action:    "remove",
		Channel:   channelInput,
		ChannelID: channelID,
		Timestamp: timestamp,
		Emoji:     emoji,
	}

	return output.Print(cmd, result)
}

func runReactionsList(cmd *cobra.Command, args []string) error {
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

	// Get reactions
	result, err := cmdCtx.Client.GetReactions(cmdCtx.Ctx, channelID, timestamp)
	if err != nil {
		return fmt.Errorf("get reactions: %w", err)
	}

	// Set the channel name in the result for human-readable output
	result.Channel = channelInput

	return output.Print(cmd, result)
}
