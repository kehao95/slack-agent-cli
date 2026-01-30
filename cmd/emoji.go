package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/output"
	"github.com/contentsquare/slack-cli/internal/slack"
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
  slack-cli emoji list

  # List with JSON output
  slack-cli emoji list --json`,
	RunE: runEmojiList,
}

func init() {
	rootCmd.AddCommand(emojiCmd)
	emojiCmd.AddCommand(emojiListCmd)
}

func runEmojiList(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	client := slack.New(cfg.UserToken)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// List emoji
	result, err := client.ListEmoji(ctx)
	if err != nil {
		return fmt.Errorf("list emoji: %w", err)
	}

	return output.Print(cmd, result)
}
