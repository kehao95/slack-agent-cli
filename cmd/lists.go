package cmd

import (
	"github.com/kehao95/slack-agent-cli/internal/lists"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/spf13/cobra"
)

var listsCmd = &cobra.Command{
	Use:   "lists",
	Short: "Slack List operations",
	Long:  "Inspect Slack Lists and retrieve their items.",
}

var listsItemsCmd = &cobra.Command{
	Use:   "items",
	Short: "List items from a Slack List",
	Long: `Fetch items from a Slack List via slackLists.items.list.

The --list flag accepts either a raw List ID like F0BFMJY6ZTQ or a Slack List URL
like https://workspace.slack.com/lists/T123/F0BFMJY6ZTQ.

Required scopes:
  - lists:read`,
	Example: `  # Read the latest items from a list
  slk lists items --list F0BFMJY6ZTQ

  # Read using a Slack List URL
  slk lists items --list https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ

  # Continue pagination
  slk lists items --list F0BFMJY6ZTQ --cursor "bGlzdF9pZD..."`,
	RunE: runListsItems,
}

var listsItemCmd = &cobra.Command{
	Use:   "item",
	Short: "Get one item from a Slack List",
	Long: `Fetch a single item from a Slack List via slackLists.items.info.

The --list flag accepts either a raw List ID like F0BFMJY6ZTQ or a Slack List URL
like https://workspace.slack.com/lists/T123/F0BFMJY6ZTQ.

Required scopes:
  - lists:read`,
	Example: `  # Read a single item by record id
  slk lists item --list F0BFMJY6ZTQ --id Rec018B8RR603

  # Include subscription state when available
  slk lists item --list https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ --id Rec018B8RR603 --include-is-subscribed`,
	RunE: runListsItem,
}

func init() {
	rootCmd.AddCommand(listsCmd)
	listsCmd.AddCommand(listsItemsCmd)
	listsCmd.AddCommand(listsItemCmd)

	listsItemsCmd.Flags().String("list", "", "Slack List ID or Slack List URL (required)")
	listsItemsCmd.Flags().Int("limit", 100, "Maximum items to return")
	listsItemsCmd.Flags().String("cursor", "", "Continuation cursor")
	listsItemsCmd.Flags().Bool("archived", false, "List archived items instead of active items")
	listsItemsCmd.Flags().Bool("all", false, "Fetch all pages of items")
	listsItemsCmd.MarkFlagRequired("list")

	listsItemCmd.Flags().String("list", "", "Slack List ID or Slack List URL (required)")
	listsItemCmd.Flags().String("id", "", "Slack List record/item ID (required)")
	listsItemCmd.Flags().Bool("include-is-subscribed", false, "Include subscription state when Slack provides it")
	listsItemCmd.MarkFlagRequired("list")
	listsItemCmd.MarkFlagRequired("id")
}

func runListsItems(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	listRef, _ := cmd.Flags().GetString("list")
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	archived, _ := cmd.Flags().GetBool("archived")
	allPages, _ := cmd.Flags().GetBool("all")

	service := lists.NewService(cmdCtx.Client, cmdCtx.UserResolver)
	result, err := service.ListItems(cmdCtx.Ctx, lists.Params{
		List:     listRef,
		Limit:    limit,
		Cursor:   cursor,
		Archived: archived,
		All:      allPages,
	})
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}

func runListsItem(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	listRef, _ := cmd.Flags().GetString("list")
	recordID, _ := cmd.Flags().GetString("id")
	includeSubscribed, _ := cmd.Flags().GetBool("include-is-subscribed")

	service := lists.NewService(cmdCtx.Client, cmdCtx.UserResolver)
	result, err := service.GetItem(cmdCtx.Ctx, lists.ItemParams{
		List:                listRef,
		ID:                  recordID,
		IncludeIsSubscribed: includeSubscribed,
	})
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}
