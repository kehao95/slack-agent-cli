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

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  "Test and verify Slack authentication credentials.",
}

var authTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Verify credentials work",
	Long:  "Test that the configured Slack tokens are valid by making an auth.test API call.",
	Example: `  slack-cli auth test
  slack-cli auth test --json`,
	RunE: runAuthTest,
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user identity",
	Long:  "Display information about the currently authenticated user.",
	Example: `  slack-cli auth whoami
  slack-cli auth whoami --json`,
	RunE: runAuthWhoami,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authTestCmd)
	authCmd.AddCommand(authWhoamiCmd)
}

func runAuthTest(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	client := slack.New(cfg.UserToken)

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	result, err := client.AuthTest(ctx)
	if err != nil {
		return fmt.Errorf("auth test: %w", err)
	}

	return output.Print(cmd, result)
}

func runAuthWhoami(cmd *cobra.Command, args []string) error {
	// whoami is essentially the same as test - both call auth.test
	return runAuthTest(cmd, args)
}
