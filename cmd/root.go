package cmd

import (
	"fmt"
	"os"

	"github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "slk",
		Short: "Slack for Non-Humans™",
		Long: `Slack for Non-Humans™ - Machine-first CLI for Slack. JSON by default.

Quick Start:
  1. Verify authentication:
     slk auth test

  2. Pre-warm cache (optional but recommended):
     slk cache populate channels --all
     slk cache populate users --all

  3. List recent messages:
     slk messages list --channel "#general" --limit 10

  4. Send a message:
     slk messages send --channel "#general" --text "Hello!"

Exit Codes:
  0 - Success
  1 - General error
  2 - Configuration error (missing config, invalid tokens)
  3 - Authentication error (invalid/expired tokens)
  4 - Rate limit exceeded
  5 - Network error
  6 - Permission denied (missing OAuth scopes)
  7 - Resource not found (channel, user, message)

Environment Variables:
  SLACK_USER_TOKEN     Override user token from config
  SLACK_CLI_CONFIG     Custom config file path
  SLACK_CLI_FORMAT     Default output format (json or human)`,
		Run: func(cmd *cobra.Command, args []string) {
			// Easter egg: Warn biological users about JSON output
			if term.IsTerminal(int(os.Stdout.Fd())) {
				// ANSI color codes: Yellow background + Black text for warning
				yellow := "\033[43m"
				black := "\033[30m"
				bold := "\033[1m"
				reset := "\033[0m"
				red := "\033[31m"

				fmt.Fprintf(os.Stderr, "\n%s%s%s ⚠️  WARNING ⚠️  %s\n", yellow, black, bold, reset)
				fmt.Fprintf(os.Stderr, "%s%sDetected biological presence.%s\n", red, bold, reset)
				fmt.Fprintf(os.Stderr, "Use %s--human%s if you can't read JSON.\n\n", bold, reset)
			}
			cmd.Help()
		},
	}
)

// SetVersionInfo sets version information for the CLI.
// This is called from main.go with values injected by GoReleaser.
func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}

// Execute runs the root command with proper exit code handling.
func Execute() {
	errors.Execute(rootCmd)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/slack-cli/config.json)")
	rootCmd.PersistentFlags().BoolP("human", "H", false, "human-readable output with tables and colors")
	viper.BindPFlag("output.human", rootCmd.PersistentFlags().Lookup("human"))
}
