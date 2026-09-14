package cmd

import (
	"fmt"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/output"
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
	Example: `  slk auth test
  slk auth test --human`,
	RunE: runAuthTest,
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the configured token principal",
	Long:  "Display the configured token principal and active role. This identifies the authenticated Slack account, not the person who triggered an agent task. User IDs are stable; username is the exact @handle.",
	Example: `  slk auth whoami
  slk auth whoami --human`,
	RunE: runAuthWhoami,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authTestCmd)
	authCmd.AddCommand(authWhoamiCmd)
}

func runAuthTest(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 10*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	result, err := cmdCtx.Client.AuthTest(cmdCtx.Ctx)
	if err != nil {
		return fmt.Errorf("auth test: %w", err)
	}
	result.Role = cmdCtx.AuthRole

	return output.Print(cmd, result)
}

func runAuthWhoami(cmd *cobra.Command, args []string) error {
	// whoami is essentially the same as test - both call auth.test
	return runAuthTest(cmd, args)
}
