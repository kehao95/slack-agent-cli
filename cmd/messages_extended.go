package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var messagesPermalinkCmd = &cobra.Command{Use: "permalink", Short: "Get a permanent Slack URL for a message", RunE: runMessagesPermalink}
var messagesEphemeralCmd = &cobra.Command{Use: "ephemeral", Short: "Send an ephemeral message to one user", RunE: runMessagesEphemeral}
var messagesScheduleCmd = &cobra.Command{Use: "schedule", Short: "Schedule a message", RunE: runMessagesSchedule}
var messagesScheduledCmd = &cobra.Command{Use: "scheduled", Short: "Inspect and delete scheduled messages"}
var messagesScheduledListCmd = &cobra.Command{Use: "list", Short: "List scheduled messages", RunE: runMessagesScheduledList}
var messagesScheduledDeleteCmd = &cobra.Command{Use: "delete", Short: "Delete a scheduled message", RunE: runMessagesScheduledDelete}
var messagesStreamCmd = &cobra.Command{Use: "stream", Short: "Manage an agent response stream"}
var messagesStreamStartCmd = &cobra.Command{Use: "start", Short: "Start an agent response stream", RunE: runMessagesStreamStart}
var messagesStreamAppendCmd = &cobra.Command{Use: "append", Short: "Append Markdown text to a stream", RunE: runMessagesStreamAppend}
var messagesStreamStopCmd = &cobra.Command{Use: "stop", Short: "Stop a stream, optionally with final content", RunE: runMessagesStreamStop}

func init() {
	messagesCmd.AddCommand(messagesPermalinkCmd, messagesEphemeralCmd, messagesScheduleCmd, messagesScheduledCmd, messagesStreamCmd)
	messagesScheduledCmd.AddCommand(messagesScheduledListCmd, messagesScheduledDeleteCmd)
	messagesStreamCmd.AddCommand(messagesStreamStartCmd, messagesStreamAppendCmd, messagesStreamStopCmd)

	messageTargetAndTimestampFlags(messagesPermalinkCmd)

	messageTargetFlag(messagesEphemeralCmd, true)
	messagesEphemeralCmd.Flags().String("user", "", "Recipient canonical user ID, <@ID>, or @username (required)")
	_ = messagesEphemeralCmd.MarkFlagRequired("user")
	addMessageContentFlags(messagesEphemeralCmd)
	messagesEphemeralCmd.Flags().String("thread", "", "Thread timestamp")

	messageTargetFlag(messagesScheduleCmd, true)
	messagesScheduleCmd.Flags().String("post-at", "", "Delivery time: Unix seconds, RFC3339, or relative duration such as 10m (required)")
	_ = messagesScheduleCmd.MarkFlagRequired("post-at")
	addMessageContentFlags(messagesScheduleCmd)
	messagesScheduleCmd.Flags().String("thread", "", "Thread timestamp")

	messageTargetFlag(messagesScheduledListCmd, false)
	messagesScheduledListCmd.Flags().String("cursor", "", "Continuation cursor")
	messagesScheduledListCmd.Flags().Int("limit", 100, "Maximum messages per page")
	messagesScheduledListCmd.Flags().String("since", "", "Messages scheduled after this time")
	messagesScheduledListCmd.Flags().String("until", "", "Messages scheduled before this time")
	messagesScheduledListCmd.Flags().Bool("all", false, "Fetch all pages")
	messagesScheduledListCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	messagesScheduledListCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")

	messageTargetFlag(messagesScheduledDeleteCmd, true)
	messagesScheduledDeleteCmd.Flags().String("id", "", "Scheduled message ID (required)")
	_ = messagesScheduledDeleteCmd.MarkFlagRequired("id")

	messageTargetFlag(messagesStreamStartCmd, true)
	messagesStreamStartCmd.Flags().String("thread", "", "Thread timestamp for the stream")
	messagesStreamStartCmd.Flags().String("recipient-user", "", "Recipient canonical user ID, <@ID>, or @username for agent streams")
	messagesStreamStartCmd.Flags().String("recipient-team", "", "Recipient Slack team ID for agent streams")

	messageTargetAndTimestampFlags(messagesStreamAppendCmd)
	messagesStreamAppendCmd.Flags().StringP("mrkdwn", "m", "", "Markdown chunk, or - to read stdin (required)")
	_ = messagesStreamAppendCmd.MarkFlagRequired("mrkdwn")

	messageTargetAndTimestampFlags(messagesStreamStopCmd)
	messagesStreamStopCmd.Flags().StringP("mrkdwn", "m", "", "Optional final Markdown text, or - to read stdin")
	messagesStreamStopCmd.Flags().String("blocks", "", "Optional final Block Kit JSON, @file, or - for stdin")
}

func messageTargetFlag(command *cobra.Command, required bool) {
	description := "Conversation ID, #name, @user, or Slack URL"
	if required {
		description += " (required)"
	}
	command.Flags().StringP("channel", "c", "", description)
	if required {
		_ = command.MarkFlagRequired("channel")
	}
}

func messageTargetAndTimestampFlags(command *cobra.Command) {
	messageTargetFlag(command, true)
	command.Flags().String("ts", "", "Message timestamp (required)")
	_ = command.MarkFlagRequired("ts")
}

func addMessageContentFlags(command *cobra.Command) {
	command.Flags().StringP("mrkdwn", "m", "", "Slack mrkdwn text, or - to read stdin")
	command.Flags().StringP("text", "t", "", "Plain text, or - to read stdin")
	command.Flags().String("blocks", "", "Block Kit JSON, @file, or - for stdin")
}

func runMessagesPermalink(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	timestamp, _ := cmd.Flags().GetString("ts")
	result, err := cmdCtx.Client.GetMessagePermalink(cmdCtx.Ctx, channelID, timestamp)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesEphemeral(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userRef, _ := cmd.Flags().GetString("user")
	userID, err := cmdCtx.ResolveUser(userRef)
	if err != nil {
		return err
	}
	content, err := readMessageContentFlags(cmd, true)
	if err != nil {
		return err
	}
	thread, _ := cmd.Flags().GetString("thread")
	result, err := cmdCtx.Client.PostEphemeralMessage(cmdCtx.Ctx, channelID, userID, slack.PostMessageOptions{
		Text: content.Text, Blocks: content.Blocks, ThreadTS: thread,
		UnfurlLinks: true, UnfurlMedia: true, AsUser: cmdCtx.AuthRole == config.RoleUser,
	})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesSchedule(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	content, err := readMessageContentFlags(cmd, true)
	if err != nil {
		return err
	}
	postAtInput, _ := cmd.Flags().GetString("post-at")
	postAt, err := normalizeScheduleTime(postAtInput, time.Now())
	if err != nil {
		return err
	}
	thread, _ := cmd.Flags().GetString("thread")
	result, err := cmdCtx.Client.ScheduleMessage(cmdCtx.Ctx, channelID, postAt, slack.PostMessageOptions{
		Text: content.Text, Blocks: content.Blocks, ThreadTS: thread,
		UnfurlLinks: true, UnfurlMedia: true, AsUser: cmdCtx.AuthRole == config.RoleUser,
	})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesScheduledList(cmd *cobra.Command, _ []string) error {
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
	channelRef, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if strings.TrimSpace(channelRef) != "" {
		channelID, err = cmdCtx.ResolveChannel(channelRef)
		if err != nil {
			return err
		}
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 || limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100")
	}
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	oldest, latest, err := slack.ParseTimeRange(since, until)
	if err != nil {
		return err
	}
	result, err := slack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*slack.ScheduledMessagesResult, error) {
		return cmdCtx.Client.ListScheduledMessages(cmdCtx.Ctx, channelID, cursor, oldest, latest, limit)
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for all && result.NextCursor != "" {
		if seen[result.NextCursor] {
			return fmt.Errorf("Slack returned repeated scheduled-message cursor %q", result.NextCursor)
		}
		seen[result.NextCursor] = true
		if err := slack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		nextCursor := result.NextCursor
		page, err := slack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*slack.ScheduledMessagesResult, error) {
			return cmdCtx.Client.ListScheduledMessages(cmdCtx.Ctx, channelID, nextCursor, oldest, latest, limit)
		})
		if err != nil {
			return err
		}
		result.Messages = append(result.Messages, page.Messages...)
		result.NextCursor = page.NextCursor
	}
	return output.Print(cmd, result)
}

func runMessagesScheduledDelete(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	id, _ := cmd.Flags().GetString("id")
	result, err := cmdCtx.Client.DeleteScheduledMessage(cmdCtx.Ctx, channelID, id, cmdCtx.AuthRole == config.RoleUser)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesStreamStart(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	thread, _ := cmd.Flags().GetString("thread")
	recipientRef, _ := cmd.Flags().GetString("recipient-user")
	recipientTeam, _ := cmd.Flags().GetString("recipient-team")
	recipientUser := ""
	if strings.TrimSpace(recipientRef) != "" {
		recipientUser, err = cmdCtx.ResolveUser(recipientRef)
		if err != nil {
			return err
		}
	}
	result, err := cmdCtx.Client.StartMessageStream(cmdCtx.Ctx, channelID, slack.StreamStartOptions{
		ThreadTS: thread, RecipientTeamID: recipientTeam, RecipientUserID: recipientUser,
	})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesStreamAppend(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	timestamp, _ := cmd.Flags().GetString("ts")
	markdown, _ := cmd.Flags().GetString("mrkdwn")
	if markdown == "-" {
		markdown, err = readRequiredStdin("mrkdwn")
		if err != nil {
			return err
		}
	}
	result, err := cmdCtx.Client.AppendMessageStream(cmdCtx.Ctx, channelID, timestamp, markdown)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runMessagesStreamStop(cmd *cobra.Command, _ []string) error {
	cmdCtx, channelID, err := messageCommandContext(cmd)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	timestamp, _ := cmd.Flags().GetString("ts")
	markdown, _ := cmd.Flags().GetString("mrkdwn")
	blocksJSON, _ := cmd.Flags().GetString("blocks")
	if markdown == "-" && blocksJSON == "-" {
		return fmt.Errorf("only one input may read stdin")
	}
	if markdown == "-" {
		markdown, err = readRequiredStdin("mrkdwn")
		if err != nil {
			return err
		}
	}
	blocks, err := parseBlocksJSON(blocksJSON)
	if err != nil {
		return err
	}
	result, err := cmdCtx.Client.StopMessageStream(cmdCtx.Ctx, channelID, timestamp, slack.StreamUpdateOptions{Markdown: markdown, Blocks: blocks})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

type messageContent struct {
	Text   string
	Blocks []slackapi.Block
}

func readMessageContentFlags(cmd *cobra.Command, required bool) (messageContent, error) {
	mrkdwn, _ := cmd.Flags().GetString("mrkdwn")
	plain, _ := cmd.Flags().GetString("text")
	blocksJSON, _ := cmd.Flags().GetString("blocks")
	stdinReaders := 0
	if mrkdwn == "-" {
		stdinReaders++
	}
	if plain == "-" {
		stdinReaders++
	}
	if blocksJSON == "-" {
		stdinReaders++
	}
	if stdinReaders > 1 {
		return messageContent{}, fmt.Errorf("only one input may read stdin")
	}
	var err error
	if mrkdwn == "-" {
		mrkdwn, err = readRequiredStdin("mrkdwn")
		if err != nil {
			return messageContent{}, err
		}
	}
	if plain == "-" {
		plain, err = readRequiredStdin("text")
		if err != nil {
			return messageContent{}, err
		}
	}
	blocks, err := parseBlocksJSON(blocksJSON)
	if err != nil {
		return messageContent{}, err
	}
	count := 0
	if mrkdwn != "" {
		count++
	}
	if plain != "" {
		count++
	}
	if cmd.Flags().Changed("blocks") {
		count++
	}
	if required && count != 1 {
		return messageContent{}, fmt.Errorf("choose exactly one message input: --mrkdwn, --text, or --blocks")
	}
	if count > 1 {
		return messageContent{}, fmt.Errorf("choose at most one message input: --mrkdwn, --text, or --blocks")
	}
	if mrkdwn != "" {
		plain = mrkdwn
	}
	return messageContent{Text: plain, Blocks: blocks}, nil
}

func messageCommandContext(cmd *cobra.Command) (*CommandContext, string, error) {
	cmdCtx, err := NewCommandContext(cmd, 0)
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

func normalizeScheduleTime(value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("post-at is required")
	}
	var postAt time.Time
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		postAt = time.Unix(unix, 0)
	} else if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		postAt = parsed
	} else if duration, err := time.ParseDuration(value); err == nil {
		postAt = now.Add(duration)
	} else {
		return "", fmt.Errorf("invalid post-at %q: use Unix seconds, RFC3339, or a relative duration", value)
	}
	if !postAt.After(now) {
		return "", fmt.Errorf("post-at must be in the future")
	}
	return strconv.FormatInt(postAt.Unix(), 10), nil
}
