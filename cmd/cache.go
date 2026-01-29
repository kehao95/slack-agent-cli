package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	slackapi "github.com/slack-go/slack"

	"github.com/contentsquare/slack-cli/internal/cache"
	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/slack"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage metadata cache",
	Long:  "Populate, inspect, and clear the local metadata cache for channels and users.",
}

var cachePopulateCmd = &cobra.Command{
	Use:   "populate <channels|users>",
	Short: "Populate cache incrementally",
	Long: `Fetch metadata from Slack and save to local cache.

By default, fetches one page at a time and saves progress. Use --all to
fetch everything (with rate limiting). If interrupted, the next run
resumes from where it left off.`,
	Example: `  # Fetch one page of channels (200 items)
  slack-cli cache populate channels

  # Fetch all channels with progress output
  slack-cli cache populate channels --all

  # Fetch all users
  slack-cli cache populate users --all

  # Custom page size and delay
  slack-cli cache populate channels --all --page-size 100 --page-delay 2s`,
	Args: cobra.ExactArgs(1),
	RunE: runCachePopulate,
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cache status",
	Long:  "Display information about cached channels and users.",
	RunE:  runCacheStatus,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear [channels|users]",
	Short: "Clear cache",
	Long:  "Remove cached data. Specify 'channels' or 'users', or omit to clear all.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCacheClear,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cachePopulateCmd)
	cacheCmd.AddCommand(cacheStatusCmd)
	cacheCmd.AddCommand(cacheClearCmd)

	cachePopulateCmd.Flags().Bool("all", false, "Fetch all pages (with rate limiting)")
	cachePopulateCmd.Flags().Int("page-size", 200, "Items per page")
	cachePopulateCmd.Flags().Duration("page-delay", time.Second, "Delay between pages")
	cachePopulateCmd.Flags().Bool("quiet", false, "Suppress progress output")
}

// channelFetcherAdapter adapts APIClient to cache.ChannelFetcher interface.
type channelFetcherAdapter struct {
	client *slack.APIClient
}

func (a *channelFetcherAdapter) ListChannels(ctx context.Context, cursor string, limit int) ([]slackapi.Channel, string, int, error) {
	return a.client.ListChannelsPaginated(ctx, cursor, limit)
}

// userFetcherAdapter adapts APIClient to cache.UserFetcher interface.
type userFetcherAdapter struct {
	client *slack.APIClient
}

func (a *userFetcherAdapter) ListUsers(ctx context.Context, cursor string, limit int) ([]slackapi.User, string, error) {
	return a.client.ListUsers(ctx, cursor, limit)
}

func runCachePopulate(cmd *cobra.Command, args []string) error {
	target := args[0]
	if target != "channels" && target != "users" {
		return fmt.Errorf("invalid target: %s (must be 'channels' or 'users')", target)
	}

	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	client := slack.New(cfg.BotToken)

	fetchAll, _ := cmd.Flags().GetBool("all")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	quiet, _ := cmd.Flags().GetBool("quiet")

	popCfg := cache.PopulateConfig{
		PageSize:  pageSize,
		PageDelay: pageDelay,
		FetchAll:  fetchAll,
	}
	if !quiet {
		popCfg.Output = os.Stdout
	}

	// Use longer timeout for --all mode
	timeout := 30 * time.Second
	if fetchAll {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	var result cache.PopulateResult

	switch target {
	case "channels":
		fmt.Fprintf(os.Stdout, "Populating channels cache...\n")
		result, err = cacheStore.PopulateChannels(ctx, &channelFetcherAdapter{client}, popCfg)
	case "users":
		fmt.Fprintf(os.Stdout, "Populating users cache...\n")
		result, err = cacheStore.PopulateUsers(ctx, &userFetcherAdapter{client}, popCfg)
	}

	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		return err
	}

	// Print summary
	status := "partial"
	if result.Complete {
		status = "complete"
	}
	fmt.Fprintf(os.Stdout, "\nResult: %d %s cached (%s)\n", result.Count, target, status)
	if !result.Complete && result.NextCursor != "" {
		fmt.Fprintf(os.Stdout, "Run again to continue fetching.\n")
	}

	if err == context.DeadlineExceeded {
		fmt.Fprintf(os.Stdout, "Timeout reached. Progress saved. Run again to continue.\n")
	}

	return nil
}

func runCacheStatus(cmd *cobra.Command, args []string) error {
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	fmt.Println("Cache Status")
	fmt.Println("============")

	for _, key := range []string{cache.CacheKeyChannels, cache.CacheKeyUsers} {
		status, found := cacheStore.GetStatus(key)
		if !found {
			fmt.Printf("  %s: not cached\n", key)
			continue
		}

		state := "complete"
		if !status.Complete {
			state = "partial"
		}
		if status.Expired {
			state += " (expired)"
		}

		age := time.Since(status.FetchedAt).Round(time.Minute)
		fmt.Printf("  %s: %d items (%s, fetched %v ago)\n", key, status.Count, state, age)
		if !status.Complete && status.NextCursor != "" {
			fmt.Printf("    next_cursor: %s...\n", truncateCursor(status.NextCursor))
		}
	}

	return nil
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	var targets []string
	if len(args) == 0 {
		targets = []string{cache.CacheKeyChannels, cache.CacheKeyUsers}
	} else {
		target := args[0]
		if target != "channels" && target != "users" {
			return fmt.Errorf("invalid target: %s (must be 'channels' or 'users')", target)
		}
		targets = []string{target}
	}

	for _, key := range targets {
		if err := cacheStore.Expire(key); err != nil {
			return fmt.Errorf("clear %s: %w", key, err)
		}
		if err := cacheStore.ExpirePartial(key); err != nil {
			return fmt.Errorf("clear %s partial: %w", key, err)
		}
		fmt.Printf("Cleared %s cache\n", key)
	}

	return nil
}

func truncateCursor(s string) string {
	if len(s) <= 20 {
		return s
	}
	return s[:20]
}
