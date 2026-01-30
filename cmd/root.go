package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
