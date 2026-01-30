package cmd

import (
	"fmt"

	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/spf13/cobra"
)

var emojiCmd = &cobra.Command{
	Use:   "emoji",
	Short: "Emoji operations",
	Long:  "List custom emoji in the workspace.",
}

var emojiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List custom emoji",
	Long:  "List all custom emoji in the Slack workspace.",
	Example: `  # List custom emoji
  slk emoji list

  # List with human-readable output
  slk emoji list --human`,
	RunE: runEmojiList,
}

func init() {
	rootCmd.AddCommand(emojiCmd)
	emojiCmd.AddCommand(emojiListCmd)
}

func runEmojiList(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	// List emoji
	result, err := cmdCtx.Client.ListEmoji(cmdCtx.Ctx)
	if err != nil {
		return fmt.Errorf("list emoji: %w", err)
	}

	return output.Print(cmd, result)
}
