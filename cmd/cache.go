package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	slackapi "github.com/slack-go/slack"

	"github.com/kehao95/slack-agent-cli/internal/cache"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
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
  slk cache populate channels

  # Fetch all channels with progress output
  slk cache populate channels --all

  # Fetch all users
  slk cache populate users --all

  # Custom page size and delay
  slk cache populate channels --all --page-size 100 --page-delay 2s`,
	Args: cobra.ExactArgs(1),
	RunE: runCachePopulate,
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cache status",
	Long: `Display information about cached channels and users.

Output (JSON):
  {
    "items": [
      {
        "key": "channels",
        "cached": true,
        "count": 42,
        "complete": true,
        "fetched_at": "2024-01-15T10:00:00Z",
        "expires_at": "2024-01-22T10:00:00Z"
      },
      {
        "key": "users",
        "cached": true,
        "count": 125,
        "complete": false,
        "fetched_at": "2024-01-15T09:30:00Z",
        "next_cursor": "dXNlcl9pZDo..."
      }
    ]
  }

Cache TTL: 7 days (automatically refreshed when stale)`,
	Example: `  # Check cache status
  slk cache status

  # Check before bulk operations
  slk cache status && slk messages list --channel "#general"`,
	RunE: runCacheStatus,
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

	fetchAll, _ := cmd.Flags().GetBool("all")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	quiet, _ := cmd.Flags().GetBool("quiet")

	// Use longer timeout for --all mode
	timeout := 30 * time.Second
	if fetchAll {
		timeout = 10 * time.Minute
	}

	cmdCtx, err := NewCommandContext(cmd, timeout)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	popCfg := cache.PopulateConfig{
		PageSize:  pageSize,
		PageDelay: pageDelay,
		FetchAll:  fetchAll,
	}
	if !quiet {
		popCfg.Output = os.Stderr
	}

	var result cache.PopulateResult

	switch target {
	case "channels":
		fmt.Fprintf(os.Stderr, "Populating channels cache...\n")
		result, err = cmdCtx.CacheStore.PopulateChannels(cmdCtx.Ctx, &channelFetcherAdapter{cmdCtx.Client}, popCfg)
	case "users":
		fmt.Fprintf(os.Stderr, "Populating users cache...\n")
		result, err = cmdCtx.CacheStore.PopulateUsers(cmdCtx.Ctx, &userFetcherAdapter{cmdCtx.Client}, popCfg)
	}

	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		return err
	}

	// Print summary
	status := "partial"
	if result.Complete {
		status = "complete"
	}
	fmt.Fprintf(os.Stderr, "\nResult: %d %s cached (%s)\n", result.Count, target, status)
	if !result.Complete && result.NextCursor != "" {
		fmt.Fprintf(os.Stderr, "Run again to continue fetching.\n")
	}

	if err == context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "Timeout reached. Progress saved. Run again to continue.\n")
	}

	return nil
}

// cacheStatusItem represents a single cache entry status
type cacheStatusItem struct {
	Key        string    `json:"key"`
	Cached     bool      `json:"cached"`
	Count      int       `json:"count,omitempty"`
	Complete   bool      `json:"complete,omitempty"`
	Expired    bool      `json:"expired,omitempty"`
	FetchedAt  time.Time `json:"fetched_at,omitempty"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// cacheStatusResponse is the response structure for cache status
type cacheStatusResponse struct {
	Items []cacheStatusItem `json:"items"`
}

// cacheStatusPrintable implements output.Printable for human-readable cache status
type cacheStatusPrintable struct {
	data cacheStatusResponse
}

func (c *cacheStatusPrintable) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.data)
}

func (c *cacheStatusPrintable) Lines() []string {
	lines := []string{
		"Cache Status",
		"============",
	}

	for _, item := range c.data.Items {
		if !item.Cached {
			lines = append(lines, fmt.Sprintf("  %s: not cached", item.Key))
			continue
		}

		state := "complete"
		if !item.Complete {
			state = "partial"
		}
		if item.Expired {
			state += " (expired)"
		}

		age := time.Since(item.FetchedAt).Round(time.Minute)
		lines = append(lines, fmt.Sprintf("  %s: %d items (%s, fetched %v ago)", item.Key, item.Count, state, age))
		if !item.Complete && item.NextCursor != "" {
			lines = append(lines, fmt.Sprintf("    next_cursor: %s...", truncateCursor(item.NextCursor)))
		}
	}

	return lines
}

func runCacheStatus(cmd *cobra.Command, args []string) error {
	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	response := cacheStatusResponse{
		Items: make([]cacheStatusItem, 0),
	}

	for _, key := range []string{cache.CacheKeyChannels, cache.CacheKeyUsers} {
		status, found := cacheStore.GetStatus(key)
		if !found {
			response.Items = append(response.Items, cacheStatusItem{
				Key:    key,
				Cached: false,
			})
			continue
		}

		item := cacheStatusItem{
			Key:        key,
			Cached:     true,
			Count:      status.Count,
			Complete:   status.Complete,
			Expired:    status.Expired,
			FetchedAt:  status.FetchedAt,
			NextCursor: status.NextCursor,
		}
		response.Items = append(response.Items, item)
	}

	return output.Print(cmd, &cacheStatusPrintable{data: response})
}

// cacheClearResult represents a single cache clear operation result
type cacheClearResult struct {
	Key     string `json:"key"`
	Cleared bool   `json:"cleared"`
}

// cacheClearResponse is the response structure for cache clear
type cacheClearResponse struct {
	Results []cacheClearResult `json:"results"`
}

// cacheClearPrintable implements output.Printable for cache clear results
type cacheClearPrintable struct {
	data cacheClearResponse
}

func (c *cacheClearPrintable) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.data)
}

func (c *cacheClearPrintable) Lines() []string {
	lines := make([]string, 0)
	for _, result := range c.data.Results {
		if result.Cleared {
			lines = append(lines, fmt.Sprintf("Cleared %s cache", result.Key))
		}
	}
	return lines
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

	response := cacheClearResponse{
		Results: make([]cacheClearResult, 0),
	}

	for _, key := range targets {
		if err := cacheStore.Expire(key); err != nil {
			return fmt.Errorf("clear %s: %w", key, err)
		}
		if err := cacheStore.ExpirePartial(key); err != nil {
			return fmt.Errorf("clear %s partial: %w", key, err)
		}
		response.Results = append(response.Results, cacheClearResult{
			Key:     key,
			Cleared: true,
		})
	}

	return output.Print(cmd, &cacheClearPrintable{data: response})
}

func truncateCursor(s string) string {
	if len(s) <= 20 {
		return s
	}
	return s[:20]
}
