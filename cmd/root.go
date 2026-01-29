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
		Use:   "slack-cli",
		Short: "CLI for interacting with Slack workspaces",
		Long:  "slack-cli enables AI coding agents to interact with Slack via shell commands.",
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/slack-cli/config.json)")
	rootCmd.PersistentFlags().Bool("json", false, "output as JSON")
	viper.BindPFlag("output.json", rootCmd.PersistentFlags().Lookup("json"))
}
