package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/kehao95/slack-agent-cli/internal/users"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "User operations",
	Long:  "List, inspect, and query Slack workspace members.",
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace members",
	Long: `List all workspace members with pagination support.

Output (JSON):
  {
    "ok": true,
    "users": [
      {
        "user_id": "U123ABC",
        "username": "@alice",
        "id": "U123ABC",
        "name": "alice",
        "real_name": "Alice Smith",
        "display_name": "Alice Smith",
        "is_bot": false,
        "is_deleted": false,
        "email": "alice@example.com",
        "email_available": true
      }
    ]
  }

Note: Set --include-bots to include bot users in results. Each user includes
email_available; false means Slack returned no email, not that the user is missing.
Email access requires users:read.email, but Slack may also omit email for other reasons.`,
	Example: `  # List one page of users
  slk users list

  # List with an explicit cursor
  slk users list --limit 50 --cursor "dXNlcl9pZDo..."

  # Follow every page with retries and a pause between requests
  slk users list --all --max-retries 3 --page-delay 250ms

  # Include bot users
  slk users list --include-bots`,
	RunE: runUsersList,
}

var usersInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get user details",
	Long: `Get detailed information about a specific user.

Output (JSON):
  {
    "ok": true,
    "user": {
      "user_id": "U123ABC",
      "username": "@alice",
      "id": "U123ABC",
      "name": "alice",
      "real_name": "Alice Smith",
      "display_name": "Alice Smith",
      "email": "alice@example.com",
      "email_available": true,
      "title": "Engineer",
      "is_bot": false,
      "is_deleted": false
    }
  }

Use 'slk users profile --user <ref>' when the full Slack profile shape is
needed.

User Identifier:
  - User ID: U123ABC (direct lookup)
  - Mention: <@U123ABC> (direct lookup)
  - Username: @alice (case-insensitive match against Slack's username field)

IDs must use their original uppercase spelling. Bare usernames, display names,
real names, and email aliases are not accepted. Use users lookup --email for email.
An explicit @ prefix always means a username, including @U123ABC.

Successful user results include email_available. An unavailable email does not
mean the user was not found. Email access requires users:read.email, but Slack
may also omit email for other reasons. No email is inferred from a username.`,
	Example: `  # Get user info by ID
  slk users info --user U123ABC

  # Get user info by username
  slk users info --user @alice`,
	RunE: runUsersInfo,
}

var usersPresenceCmd = &cobra.Command{
	Use:   "presence",
	Short: "Check user presence",
	Long:  "Check the presence status of a specific user.",
	Example: `  slk users presence --user U123ABC
  slk users presence --user @alice
  slk users presence --user U123ABC --human`,
	RunE: runUsersPresence,
}

var usersLookupCmd = &cobra.Command{Use: "lookup", Short: "Look up a user by email", Long: "Look up a user with users.lookupByEmail; requires users:read.email. A successful result reports email_available based on Slack's response and never fills a missing email from the input or username.", Example: "  slk users lookup --email alice@example.com", RunE: runUsersLookup}
var usersProfileCmd = &cobra.Command{Use: "profile", Short: "Get a user profile", Long: "Get a user profile using users.profile.get (users.profile:read). A successful result includes email_available; false means the profile was found without an email. Email access requires users:read.email, but Slack may also omit email for other reasons.", Example: "  slk users profile\n  slk users profile --user @alice --include-labels", RunE: runUsersProfile}
var usersStatusCmd = &cobra.Command{Use: "status", Short: "Get or update custom status"}
var usersStatusGetCmd = &cobra.Command{Use: "get", Short: "Get custom status", Example: "  slk users status get\n  slk users status get --user @alice", RunE: runUsersStatusGet}
var usersStatusSetCmd = &cobra.Command{Use: "set", Short: "Set custom status", Example: "  slk users status set --text 'In focus time' --emoji :headphones: --expires-in 2h\n  slk users status set --text 'OOO' --expires-at 2026-09-01T09:00:00Z", RunE: runUsersStatusSet}
var usersStatusClearCmd = &cobra.Command{Use: "clear", Short: "Clear custom status", Example: "  slk users status clear", RunE: runUsersStatusClear}
var usersConversationsCmd = &cobra.Command{Use: "conversations", Short: "List conversations for a user", Example: "  slk users conversations --user @alice --types public_channel,private_channel\n  slk users conversations --all", RunE: runUsersConversations}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersInfoCmd)
	usersCmd.AddCommand(usersPresenceCmd)
	usersCmd.AddCommand(usersLookupCmd, usersProfileCmd, usersStatusCmd, usersConversationsCmd)
	usersStatusCmd.AddCommand(usersStatusGetCmd, usersStatusSetCmd, usersStatusClearCmd)

	// users list flags
	usersListCmd.Flags().Int("limit", 100, "Maximum users per page")
	usersListCmd.Flags().String("cursor", "", "Continuation cursor for pagination")
	usersListCmd.Flags().Bool("include-bots", false, "Include bot users in results")
	usersListCmd.Flags().Bool("all", false, "Fetch all remaining pages")
	usersListCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	usersListCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")

	// users info flags
	usersInfoCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (required)")
	_ = usersInfoCmd.MarkFlagRequired("user")

	// users presence flags
	usersPresenceCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (required)")

	usersLookupCmd.Flags().String("email", "", "Email address (required)")
	_ = usersLookupCmd.MarkFlagRequired("email")
	usersProfileCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	usersProfileCmd.Flags().Bool("include-labels", false, "Include custom profile field labels")
	usersStatusGetCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	for _, command := range []*cobra.Command{usersStatusSetCmd, usersStatusClearCmd} {
		command.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	}
	usersStatusSetCmd.Flags().String("text", "", "Custom status text")
	usersStatusSetCmd.Flags().String("emoji", "", "Custom status emoji, for example :headphones:")
	usersStatusSetCmd.Flags().Duration("expires-in", 0, "Clear status after a duration, for example 2h")
	usersStatusSetCmd.Flags().String("expires-at", "", "Expiration as Unix seconds or RFC3339 timestamp")
	usersConversationsCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	usersConversationsCmd.Flags().String("types", "public_channel,private_channel,mpim,im", "Comma-separated conversation types")
	usersConversationsCmd.Flags().IntP("limit", "l", 100, "Maximum conversations per page")
	usersConversationsCmd.Flags().String("cursor", "", "Continuation cursor")
	usersConversationsCmd.Flags().Bool("all", false, "Fetch all remaining pages")
	usersConversationsCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	usersConversationsCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")
	usersConversationsCmd.Flags().Bool("include-archived", false, "Include archived conversations")
}

func runUsersList(cmd *cobra.Command, args []string) error {
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

	service := users.NewService(cmdCtx.Client)

	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	includeBots, _ := cmd.Flags().GetBool("include-bots")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	combined := &users.ListResult{OK: true, Users: []users.UserInfo{}}
	seen := map[string]bool{}
	for {
		page, err := slack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*users.ListResult, error) {
			return service.List(cmdCtx.Ctx, users.ListParams{Limit: limit, Cursor: cursor, IncludeBots: includeBots})
		})
		if err != nil {
			return err
		}
		combined.Users = append(combined.Users, page.Users...)
		combined.NextCursor = page.NextCursor
		if !all || page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return fmt.Errorf("Slack returned repeated users-list cursor %q", page.NextCursor)
		}
		seen[page.NextCursor] = true
		if err := slack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.NextCursor
	}
	return output.Print(cmd, combined)
}

func runUsersInfo(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 10*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	service := users.NewService(cmdCtx.Client)

	userInput, _ := cmd.Flags().GetString("user")
	if userInput == "" {
		return fmt.Errorf("--user flag is required")
	}

	// Resolve user ID from @username or user ID
	userID, err := resolveUserID(cmdCtx.Ctx, cmdCtx.Client, userInput)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}

	result, err := service.GetInfo(cmdCtx.Ctx, userID)
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}

func runUsersPresence(cmd *cobra.Command, args []string) error {
	cmdCtx, err := NewCommandContext(cmd, 10*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	service := users.NewService(cmdCtx.Client)

	userInput, _ := cmd.Flags().GetString("user")
	if userInput == "" {
		return fmt.Errorf("--user flag is required")
	}

	// Resolve user ID from @username or user ID
	userID, err := resolveUserID(cmdCtx.Ctx, cmdCtx.Client, userInput)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}

	result, err := service.GetPresence(cmdCtx.Ctx, userID)
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}

func runUsersLookup(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	email, _ := cmd.Flags().GetString("email")
	result, err := users.NewService(cmdCtx.Client).LookupByEmail(cmdCtx.Ctx, email)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsersProfile(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	includeLabels, _ := cmd.Flags().GetBool("include-labels")
	result, err := users.NewService(cmdCtx.Client).GetProfile(cmdCtx.Ctx, userID, includeLabels)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsersStatusGet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	result, err := users.NewService(cmdCtx.Client).GetStatus(cmdCtx.Ctx, userID)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsersStatusSet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	text, _ := cmd.Flags().GetString("text")
	emoji, _ := cmd.Flags().GetString("emoji")
	if strings.TrimSpace(text) == "" && strings.TrimSpace(emoji) == "" {
		return fmt.Errorf("provide --text or --emoji; use 'users status clear' to clear status")
	}
	expiration, err := parseStatusExpiration(cmd)
	if err != nil {
		return err
	}
	result, err := users.NewService(cmdCtx.Client).SetStatus(cmdCtx.Ctx, userID, text, emoji, expiration)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsersStatusClear(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	result, err := users.NewService(cmdCtx.Client).SetStatus(cmdCtx.Ctx, userID, "", "", 0)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsersConversations(cmd *cobra.Command, _ []string) error {
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
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	typesValue, _ := cmd.Flags().GetString("types")
	types := splitNonEmpty(typesValue)
	valid := map[string]bool{"public_channel": true, "private_channel": true, "mpim": true, "im": true}
	for _, value := range types {
		if !valid[value] {
			return fmt.Errorf("invalid conversation type %q", value)
		}
	}
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
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	service := users.NewService(cmdCtx.Client)
	combined := &users.ConversationsResult{OK: true, UserID: userID}
	seen := map[string]bool{}
	for {
		page, err := slack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*users.ConversationsResult, error) {
			return service.ListConversations(cmdCtx.Ctx, userID, types, limit, cursor, !includeArchived)
		})
		if err != nil {
			return err
		}
		combined.Conversations = append(combined.Conversations, page.Conversations...)
		combined.NextCursor = page.NextCursor
		if !all || page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return fmt.Errorf("Slack returned repeated user-conversation cursor %q", page.NextCursor)
		}
		seen[page.NextCursor] = true
		if err := slack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.NextCursor
	}
	return output.Print(cmd, combined)
}

func optionalResolvedUser(cmdCtx *CommandContext, cmd *cobra.Command) (string, error) {
	value, _ := cmd.Flags().GetString("user")
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return resolveUserID(cmdCtx.Ctx, cmdCtx.Client, value)
}

func parseStatusExpiration(cmd *cobra.Command) (int64, error) {
	duration, _ := cmd.Flags().GetDuration("expires-in")
	at, _ := cmd.Flags().GetString("expires-at")
	if duration < 0 {
		return 0, fmt.Errorf("--expires-in cannot be negative")
	}
	if duration > 0 && at != "" {
		return 0, fmt.Errorf("choose only one of --expires-in or --expires-at")
	}
	if duration > 0 {
		return time.Now().Add(duration).Unix(), nil
	}
	if at == "" {
		return 0, nil
	}
	if unix, err := strconv.ParseInt(at, 10, 64); err == nil {
		if unix < 0 {
			return 0, fmt.Errorf("--expires-at cannot be negative")
		}
		return unix, nil
	}
	parsed, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return 0, fmt.Errorf("parse --expires-at: expected Unix seconds or RFC3339: %w", err)
	}
	return parsed.Unix(), nil
}

func splitNonEmpty(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

// resolveUserID accepts canonical uppercase IDs, strict mentions, and explicit
// @usernames through the shared resolver. Names and emails are not aliases.
func resolveUserID(ctx context.Context, client *slack.APIClient, input string) (string, error) {
	userID, err := client.ResolveUserReference(ctx, input)
	if errors.Is(err, context.DeadlineExceeded) {
		return "", fmt.Errorf("resolve user %q timed out; use a Slack user ID in large workspaces: %w", input, err)
	}
	return userID, err
}
