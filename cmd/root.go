package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "slack-agent-cli",
		Short: "Slack for Non-Humans™",
		Long: `Slack for Non-Humans™

A machine-first CLI for Slack. Designed for scripts, cron jobs, and AI agents.
Humans are supported as second-class citizens via --human flag.

Output is JSON by default. All status messages go to stderr.`,
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

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/slack-agent-cli/config.json)")
	rootCmd.PersistentFlags().BoolP("human", "H", false, "human-readable output with tables and colors")
	viper.BindPFlag("output.human", rootCmd.PersistentFlags().Lookup("human"))
}
