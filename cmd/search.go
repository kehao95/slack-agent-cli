package cmd

import (
	"fmt"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/output"
	searchops "github.com/kehao95/slack-agent-cli/internal/search"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{Use: "search", Short: "Search messages and files", Long: "Unified workspace search across Slack messages, files, or both resource families."}
var searchAllCmd = newSearchResourceCommand(appslack.SearchKindAll, "Search messages and files")
var searchMessagesCmd = newSearchResourceCommand(appslack.SearchKindMessages, "Search messages")
var searchFilesCmd = newSearchResourceCommand(appslack.SearchKindFiles, "Search files")

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.AddCommand(searchAllCmd, searchMessagesCmd, searchFilesCmd)
}

func newSearchResourceCommand(kind, short string) *cobra.Command {
	command := &cobra.Command{
		Use: kind, Short: short,
		Example: fmt.Sprintf("  slk search %s --query 'deployment failed'\n  slk search %s --query 'report' --page 2 --limit 50\n  slk search %s --query 'incident' --all", kind, kind, kind),
		RunE:    func(cmd *cobra.Command, _ []string) error { return runUnifiedSearch(cmd, kind) },
	}
	command.Flags().StringP("query", "q", "", "Search query (required)")
	command.Flags().IntP("limit", "l", 20, "Results per page (maximum 100)")
	command.Flags().Int("page", 1, "Page number to fetch")
	command.Flags().Bool("all", false, "Fetch all pages starting at --page")
	command.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	command.Flags().Duration("page-delay", 0, "Delay between pagination requests")
	command.Flags().String("sort", "timestamp", "Sort by 'score' or 'timestamp'")
	command.Flags().String("sort-dir", "desc", "Sort direction 'asc' or 'desc'")
	command.Flags().Bool("highlight", false, "Include Slack search highlights")
	command.Flags().Bool("resolved-json", true, "Enrich messages with channel names and separate user metadata")
	command.Flags().Bool("raw-json", false, "Retain native Slack fields without identity enrichment")
	_ = command.MarkFlagRequired("query")
	return command
}

func runUnifiedSearch(cmd *cobra.Command, kind string) error {
	query, _ := cmd.Flags().GetString("query")
	limit, _ := cmd.Flags().GetInt("limit")
	page, _ := cmd.Flags().GetInt("page")
	all, _ := cmd.Flags().GetBool("all")
	sortBy, _ := cmd.Flags().GetString("sort")
	sortDir, _ := cmd.Flags().GetString("sort-dir")
	highlight, _ := cmd.Flags().GetBool("highlight")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if limit < 1 || limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100")
	}
	if page < 1 {
		return fmt.Errorf("--page must be at least 1")
	}
	if sortBy != "score" && sortBy != "timestamp" {
		return fmt.Errorf("invalid --sort %q: must be score or timestamp", sortBy)
	}
	if sortDir != "asc" && sortDir != "desc" {
		return fmt.Errorf("invalid --sort-dir %q: must be asc or desc", sortDir)
	}
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}

	timeout := time.Duration(0)
	if all {
		timeout = 15 * time.Minute
	}
	cmdCtx, err := NewCommandContext(cmd, timeout)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	result, err := searchops.NewService(cmdCtx.Client).Search(cmdCtx.Ctx, searchops.Params{Kind: kind, Query: query, Count: limit, Page: page, All: all, SortBy: sortBy, SortDir: sortDir, Highlight: highlight, MaxRetries: maxRetries, PageDelay: pageDelay})
	if err != nil {
		return err
	}
	if result.Messages != nil {
		rawJSON, _ := cmd.Flags().GetBool("raw-json")
		resolvedJSON, _ := cmd.Flags().GetBool("resolved-json")
		result.Messages.SetUserResolver(cmdCtx.Ctx, cmdCtx.UserResolver)
		result.Messages.SetChannelResolver(cmdCtx.Ctx, cmdCtx.ChannelResolver)
		result.Messages.SetRawJSON(rawJSON || !resolvedJSON)
	}
	return output.Print(cmd, result)
}
