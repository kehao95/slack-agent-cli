package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/channels"
	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/slack"
	"github.com/contentsquare/slack-cli/internal/watch"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for real-time messages",
	Long:  "Stream real-time messages and events via Socket Mode. Requires app token (SLACK_APP_TOKEN).",
	Example: `  # Watch all channels
  slack-cli watch

  # Watch specific channels
  slack-cli watch --channels "#general,#dev"

  # Watch with JSON output
  slack-cli watch --json

  # Watch for 5 minutes
  slack-cli watch --timeout 5m

  # Watch only reactions
  slack-cli watch --events reaction_added,reaction_removed

  # Include bot messages and thread replies
  slack-cli watch --include-bots --include-threads`,
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.Flags().StringSlice("channels", nil, "Channels to watch (default: all)")
	watchCmd.Flags().Bool("include-bots", false, "Include bot messages")
	watchCmd.Flags().Bool("include-own", false, "Include own messages")
	watchCmd.Flags().Bool("include-threads", true, "Include thread replies")
	watchCmd.Flags().Duration("timeout", 0, "Exit after duration (e.g., 60s, 5m)")
	watchCmd.Flags().StringSlice("events", []string{"message", "reaction_added", "reaction_removed"}, "Event types to watch")
	watchCmd.Flags().Bool("quiet", false, "Suppress connection status messages")
}

func runWatch(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	// Validate app token is present
	if cfg.AppToken == "" {
		return fmt.Errorf("watch requires app token (set SLACK_APP_TOKEN or add app_token to config)")
	}

	// Get flags
	channelInputs, _ := cmd.Flags().GetStringSlice("channels")
	includeBots, _ := cmd.Flags().GetBool("include-bots")
	includeOwn, _ := cmd.Flags().GetBool("include-own")
	includeThreads, _ := cmd.Flags().GetBool("include-threads")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	events, _ := cmd.Flags().GetStringSlice("events")
	quiet, _ := cmd.Flags().GetBool("quiet")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	ctx := cmd.Context()

	// Resolve channel names to IDs if specified
	var channelIDs []string
	if len(channelInputs) > 0 {
		// Initialize cache for channel resolution
		cacheStore, err := cache.DefaultStore()
		if err != nil {
			return fmt.Errorf("init cache: %w", err)
		}

		client := slack.New(cfg.BotToken)
		channelResolver := channels.NewCachedResolver(client, cacheStore)

		resolveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		for _, channelInput := range channelInputs {
			// Trim spaces and remove # prefix if present
			channelInput = strings.TrimSpace(channelInput)
			channelInput = strings.TrimPrefix(channelInput, "#")

			channelID, err := channelResolver.ResolveID(resolveCtx, channelInput)
			if err != nil {
				return fmt.Errorf("resolve channel %s: %w", channelInput, err)
			}
			channelIDs = append(channelIDs, channelID)
		}
	}

	// Get bot user ID for filtering own messages
	client := slack.New(cfg.BotToken)
	authTestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	authResp, err := client.AuthTest(authTestCtx)
	if err != nil {
		return fmt.Errorf("get bot user ID: %w", err)
	}

	// Create and run watcher
	watcher := watch.New(cfg.BotToken, cfg.AppToken, watch.Config{
		Channels:       channelIDs,
		IncludeBots:    includeBots,
		IncludeOwn:     includeOwn,
		IncludeThreads: includeThreads,
		Events:         events,
		JSONOutput:     jsonOutput,
		Quiet:          quiet,
		Timeout:        timeout,
		BotUserID:      authResp.UserID,
	})

	return watcher.Run(ctx)
}
