package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/channels"
	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var conversationsCmd = &cobra.Command{
	Use:     "conversations",
	Aliases: []string{"conversation"},
	Short:   "Conversation operations",
	Long:    "Manage public channels, private channels, DMs, and group DMs through Slack's conversations API.",
}

var conversationsListCmd = &cobra.Command{Use: "list", Short: "List conversations", RunE: runConversationsList}
var conversationsInfoCmd = &cobra.Command{Use: "info", Short: "Get conversation details", RunE: runConversationsInfo}
var conversationsCreateCmd = &cobra.Command{Use: "create", Short: "Create a public or private channel", RunE: runConversationsCreate}
var conversationsArchiveCmd = &cobra.Command{Use: "archive", Short: "Archive a conversation", RunE: runConversationsArchive}
var conversationsUnarchiveCmd = &cobra.Command{Use: "unarchive", Short: "Unarchive a conversation", RunE: runConversationsUnarchive}
var conversationsRenameCmd = &cobra.Command{Use: "rename", Short: "Rename a conversation", RunE: runConversationsRename}
var conversationsTopicCmd = &cobra.Command{Use: "topic", Short: "Set a conversation topic", RunE: runConversationsTopic}
var conversationsPurposeCmd = &cobra.Command{Use: "purpose", Short: "Set a conversation purpose", RunE: runConversationsPurpose}
var conversationsMembersCmd = &cobra.Command{Use: "members", Short: "List conversation members", RunE: runConversationsMembers}
var conversationsInviteCmd = &cobra.Command{Use: "invite", Short: "Invite users to a conversation", RunE: runConversationsInvite}
var conversationsKickCmd = &cobra.Command{Use: "kick", Short: "Remove a user from a conversation", RunE: runConversationsKick}
var conversationsOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open or resume a DM or group DM",
	Long:  "Open a DM/MPDM from user references, or resume a DM by conversation ID. User references accept canonical uppercase IDs, complete <@ID> mentions, or explicit @usernames. Display names and bare handles are not accepted.",
	RunE:  runConversationsOpen,
}
var conversationsCloseCmd = &cobra.Command{Use: "close", Short: "Close a DM or group DM", RunE: runConversationsClose}
var conversationsMarkCmd = &cobra.Command{Use: "mark", Short: "Set a conversation's read marker", RunE: runConversationsMark}
var conversationsJoinCmd = &cobra.Command{Use: "join", Short: "Join a public conversation", RunE: runChannelsJoin}
var conversationsLeaveCmd = &cobra.Command{Use: "leave", Short: "Leave a conversation", RunE: runChannelsLeave}

func init() {
	rootCmd.AddCommand(conversationsCmd)
	conversationsCmd.AddCommand(
		conversationsListCmd, conversationsInfoCmd, conversationsCreateCmd,
		conversationsArchiveCmd, conversationsUnarchiveCmd, conversationsRenameCmd,
		conversationsTopicCmd, conversationsPurposeCmd, conversationsMembersCmd,
		conversationsInviteCmd, conversationsKickCmd, conversationsOpenCmd,
		conversationsCloseCmd, conversationsMarkCmd, conversationsJoinCmd,
		conversationsLeaveCmd,
	)

	conversationsListCmd.Flags().Bool("include-archived", false, "Include archived conversations")
	conversationsListCmd.Flags().Int("limit", 200, "Maximum conversations per page")
	conversationsListCmd.Flags().String("cursor", "", "Continuation cursor")
	conversationsListCmd.Flags().StringSlice("types", []string{"public_channel"}, "Conversation types: public_channel,private_channel,im,mpim")
	conversationsListCmd.Flags().Bool("all", false, "Fetch all pages")
	conversationsListCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	conversationsListCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")

	conversationChannelFlag(conversationsInfoCmd)
	conversationsInfoCmd.Flags().Bool("include-locale", false, "Include locale metadata")
	conversationsInfoCmd.Flags().Bool("include-num-members", true, "Include member count")

	conversationsCreateCmd.Flags().String("name", "", "Conversation name (required)")
	conversationsCreateCmd.Flags().Bool("private", false, "Create a private channel")
	_ = conversationsCreateCmd.MarkFlagRequired("name")

	for _, command := range []*cobra.Command{conversationsArchiveCmd, conversationsUnarchiveCmd, conversationsCloseCmd, conversationsJoinCmd, conversationsLeaveCmd} {
		conversationChannelFlag(command)
	}

	conversationChannelFlag(conversationsRenameCmd)
	conversationsRenameCmd.Flags().String("name", "", "New conversation name (required)")
	_ = conversationsRenameCmd.MarkFlagRequired("name")

	conversationChannelFlag(conversationsTopicCmd)
	conversationsTopicCmd.Flags().String("value", "", "New topic; empty clears it")
	conversationChannelFlag(conversationsPurposeCmd)
	conversationsPurposeCmd.Flags().String("value", "", "New purpose; empty clears it")

	conversationChannelFlag(conversationsMembersCmd)
	conversationsMembersCmd.Flags().Int("limit", 200, "Maximum members per page")
	conversationsMembersCmd.Flags().String("cursor", "", "Continuation cursor")
	conversationsMembersCmd.Flags().Bool("all", false, "Fetch all pages")
	conversationsMembersCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	conversationsMembersCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")

	conversationChannelFlag(conversationsInviteCmd)
	conversationsInviteCmd.Flags().StringSlice("users", nil, "Canonical user IDs, <@ID> mentions, or @usernames to invite (required)")
	_ = conversationsInviteCmd.MarkFlagRequired("users")

	conversationChannelFlag(conversationsKickCmd)
	conversationsKickCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username to remove (required)")
	_ = conversationsKickCmd.MarkFlagRequired("user")

	conversationsOpenCmd.Flags().String("channel", "", "Existing DM or MPDM conversation ID")
	conversationsOpenCmd.Flags().StringSlice("users", nil, "Canonical user IDs, <@ID> mentions, or @usernames for a DM or MPDM")

	conversationChannelFlag(conversationsMarkCmd)
	conversationsMarkCmd.Flags().String("ts", "", "Message timestamp to mark read through (required)")
	_ = conversationsMarkCmd.MarkFlagRequired("ts")
}

func conversationChannelFlag(command *cobra.Command) {
	command.Flags().StringP("channel", "c", "", "Conversation ID, #name, @user, or Slack URL (required)")
	_ = command.MarkFlagRequired("channel")
}

func runConversationsList(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	timeout := time.Duration(0)
	if all {
		timeout = 15 * time.Minute
	}
	cmdCtx, err := NewCommandContext(cmd, timeout)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 || limit > 999 {
		return fmt.Errorf("--limit must be between 1 and 999")
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	types, _ := cmd.Flags().GetStringSlice("types")
	validTypes := map[string]bool{"public_channel": true, "private_channel": true, "im": true, "mpim": true}
	for _, conversationType := range types {
		if !validTypes[conversationType] {
			return fmt.Errorf("invalid conversation type %q", conversationType)
		}
	}
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	service := channels.NewService(cmdCtx.Client)
	result := channels.ListResult{}
	seen := map[string]bool{}
	for {
		page, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (channels.ListResult, error) {
			return service.List(cmdCtx.Ctx, channels.ListParams{Limit: limit, Cursor: cursor, IncludeArchived: includeArchived, Types: types})
		})
		if err != nil {
			return err
		}
		result.Channels = append(result.Channels, page.Channels...)
		result.NextCursor = page.NextCursor
		if !all || page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return fmt.Errorf("Slack returned repeated conversation cursor %q", page.NextCursor)
		}
		seen[page.NextCursor] = true
		if err := appslack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.NextCursor
	}
	if all {
		result.NextCursor = ""
	}
	return output.Print(cmd, result)
}

func runConversationsInfo(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := conversationCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	includeLocale, _ := cmd.Flags().GetBool("include-locale")
	includeNumMembers, _ := cmd.Flags().GetBool("include-num-members")
	result, err := cmdCtx.Client.ConversationInfo(cmdCtx.Ctx, channelID, includeLocale, includeNumMembers)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runConversationsCreate(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	name, _ := cmd.Flags().GetString("name")
	private, _ := cmd.Flags().GetBool("private")
	result, err := cmdCtx.Client.CreateConversation(cmdCtx.Ctx, name, private)
	if err != nil {
		return err
	}
	invalidateConversationCache(cmdCtx)
	return output.Print(cmd, result)
}

func runConversationsArchive(cmd *cobra.Command, _ []string) error {
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.ArchiveConversation(ctx.Ctx, id)
	})
}
func runConversationsUnarchive(cmd *cobra.Command, _ []string) error {
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.UnarchiveConversation(ctx.Ctx, id)
	})
}
func runConversationsClose(cmd *cobra.Command, _ []string) error {
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.CloseConversation(ctx.Ctx, id)
	})
}

func runConversationsRename(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.RenameConversation(ctx.Ctx, id, name)
	})
}
func runConversationsTopic(cmd *cobra.Command, _ []string) error {
	value, _ := cmd.Flags().GetString("value")
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.SetConversationTopic(ctx.Ctx, id, value)
	})
}
func runConversationsPurpose(cmd *cobra.Command, _ []string) error {
	value, _ := cmd.Flags().GetString("value")
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.SetConversationPurpose(ctx.Ctx, id, value)
	})
}

func runConversationsMembers(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	timeout := time.Duration(0)
	if all {
		timeout = 15 * time.Minute
	}
	cmdCtx, channelID, err := conversationCommandContextWithTimeout(cmd, timeout)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 || limit > 999 {
		return fmt.Errorf("--limit must be between 1 and 999")
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	result, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*appslack.ConversationMembersResult, error) {
		return cmdCtx.Client.ListConversationMembers(cmdCtx.Ctx, channelID, cursor, limit)
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for all && result.NextCursor != "" {
		if seen[result.NextCursor] {
			return fmt.Errorf("Slack returned repeated member cursor %q", result.NextCursor)
		}
		seen[result.NextCursor] = true
		if err := appslack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		nextCursor := result.NextCursor
		page, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*appslack.ConversationMembersResult, error) {
			return cmdCtx.Client.ListConversationMembers(cmdCtx.Ctx, channelID, nextCursor, limit)
		})
		if err != nil {
			return err
		}
		result.Members = append(result.Members, page.Members...)
		result.NextCursor = page.NextCursor
	}
	return output.Print(cmd, result)
}

func runConversationsInvite(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := conversationCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	refs, _ := cmd.Flags().GetStringSlice("users")
	users, err := resolveUserReferences(cmdCtx, refs)
	if err != nil {
		return err
	}
	result, err := cmdCtx.Client.InviteConversationUsers(cmdCtx.Ctx, channelID, users)
	if err != nil {
		return err
	}
	invalidateConversationCache(cmdCtx)
	return output.Print(cmd, result)
}

func runConversationsKick(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := conversationCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	ref, _ := cmd.Flags().GetString("user")
	userID, err := cmdCtx.ResolveUser(ref)
	if err != nil {
		return err
	}
	result, err := cmdCtx.Client.KickConversationUser(cmdCtx.Ctx, channelID, userID)
	if err != nil {
		return err
	}
	invalidateConversationCache(cmdCtx)
	return output.Print(cmd, result)
}

func runConversationsOpen(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	channel, _ := cmd.Flags().GetString("channel")
	refs, _ := cmd.Flags().GetStringSlice("users")
	if strings.TrimSpace(channel) != "" && len(refs) > 0 {
		return fmt.Errorf("choose exactly one of --channel or --users")
	}
	if strings.TrimSpace(channel) == "" && len(refs) == 0 {
		return fmt.Errorf("one of --channel or --users is required")
	}
	var users []string
	if len(refs) > 0 {
		users, err = resolveUserReferences(cmdCtx, refs)
		if err != nil {
			return err
		}
	}
	result, err := cmdCtx.Client.OpenConversation(cmdCtx.Ctx, strings.TrimSpace(channel), users)
	if err != nil {
		return err
	}
	invalidateConversationCache(cmdCtx)
	return output.Print(cmd, result)
}

func runConversationsMark(cmd *cobra.Command, _ []string) error {
	timestamp, _ := cmd.Flags().GetString("ts")
	return runConversationIDMutation(cmd, func(ctx *CommandContext, id string) (interface{}, error) {
		return ctx.Client.MarkConversationRead(ctx.Ctx, id, timestamp)
	})
}

func conversationCommandContext(cmd *cobra.Command) (*CommandContext, string, error) {
	return conversationCommandContextWithTimeout(cmd, 0)
}

func conversationCommandContextWithTimeout(cmd *cobra.Command, timeout time.Duration) (*CommandContext, string, error) {
	cmdCtx, err := NewCommandContext(cmd, timeout)
	if err != nil {
		return nil, "", err
	}
	channel, _ := cmd.Flags().GetString("channel")
	channelID, err := cmdCtx.ResolveChannel(channel)
	if err != nil {
		cmdCtx.Close()
		return nil, "", err
	}
	return cmdCtx, channelID, nil
}

func runConversationIDMutation(cmd *cobra.Command, mutate func(*CommandContext, string) (interface{}, error)) error {
	cmdCtx, channelID, err := conversationCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	result, err := mutate(cmdCtx, channelID)
	if err != nil {
		return err
	}
	invalidateConversationCache(cmdCtx)
	return output.Print(cmd, result)
}

func resolveUserReferences(cmdCtx *CommandContext, refs []string) ([]string, error) {
	resolved, err := cmdCtx.Client.ResolveUserReferences(cmdCtx.Ctx, refs)
	if err != nil {
		return nil, err
	}
	users := make([]string, 0, len(resolved))
	seen := map[string]bool{}
	for _, id := range resolved {
		if !seen[id] {
			seen[id] = true
			users = append(users, id)
		}
	}
	return users, nil
}

func invalidateConversationCache(cmdCtx *CommandContext) {
	if cmdCtx != nil && cmdCtx.ChannelResolver != nil {
		_ = cmdCtx.ChannelResolver.RefreshCache(cmdCtx.Ctx)
	}
}
