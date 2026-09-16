package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"

	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	usersops "github.com/kehao95/slack-agent-cli/internal/users"
)

// Search commands intentionally use deterministic directory-backed filtering.
// They do not claim semantic or relevance-ranked search.

var usersSearchCmd = &cobra.Command{
	Use:     "search",
	Short:   "Search workspace users",
	Long:    "Search users by partial name/display name or exact email; a Slack user ID matches exactly. Results retain stable user IDs.",
	Example: "  slk users search --query alice\n  slk users search --query alice@example.com",
	RunE:    runUsersSearch,
}

var conversationsSearchCmd = &cobra.Command{
	Use:     "search",
	Short:   "Search accessible conversations",
	Long:    "Search conversation names and descriptions (Slack purpose/topic) with deterministic local substring matching.",
	Example: "  slk conversations search --query incident\n  slk conversations search --description on-call",
	RunE:    runConversationsSearch,
}

type userSearchResult struct {
	OK         bool                `json:"ok"`
	Query      string              `json:"query"`
	Users      []usersops.UserInfo `json:"users"`
	Matches    int                 `json:"matches"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type conversationSearchResult struct {
	OK            bool            `json:"ok"`
	Query         string          `json:"query,omitempty"`
	Name          string          `json:"name,omitempty"`
	Description   string          `json:"description,omitempty"`
	Conversations []slack.Channel `json:"conversations"`
	Matches       int             `json:"matches"`
	NextCursor    string          `json:"next_cursor,omitempty"`
}

type conversationPage struct {
	channels   []slack.Channel
	nextCursor string
}

func init() {
	usersCmd.AddCommand(usersSearchCmd)
	conversationsCmd.AddCommand(conversationsSearchCmd)
	configureUsersSearchFlags(usersSearchCmd)
	configureConversationSearchFlags(conversationsSearchCmd)
}

func configureUsersSearchFlags(command *cobra.Command) {
	command.Flags().String("query", "", "Partial name/display name, exact email, or exact user ID (required)")
	command.Flags().Bool("include-bots", false, "Include bot users")
	command.Flags().Bool("include-deleted", false, "Include deleted users")
	command.Flags().Int("limit", 200, "Users per directory page")
	command.Flags().String("cursor", "", "Start at a users.list continuation cursor")
	command.Flags().Bool("all", true, "Follow all remaining directory pages")
	command.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	command.Flags().Duration("page-delay", 0, "Delay between directory pages")
	_ = command.MarkFlagRequired("query")
}

func configureConversationSearchFlags(command *cobra.Command) {
	command.Flags().String("query", "", "Substring matched against name, topic, or purpose")
	command.Flags().String("name", "", "Substring matched against conversation name")
	command.Flags().String("description", "", "Substring matched against purpose or topic")
	command.Flags().StringSlice("types", []string{"public_channel"}, "Conversation types, comma-separated")
	command.Flags().Bool("include-archived", false, "Include archived conversations")
	command.Flags().Int("limit", 200, "Conversations per directory page")
	command.Flags().String("cursor", "", "Start at a conversations.list continuation cursor")
	command.Flags().Bool("all", true, "Follow all remaining directory pages")
	command.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	command.Flags().Duration("page-delay", 0, "Delay between directory pages")
}

func runUsersSearch(cmd *cobra.Command, _ []string) error {
	query, _ := cmd.Flags().GetString("query")
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("--query is required")
	}
	limit, _ := cmd.Flags().GetInt("limit")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	includeBots, _ := cmd.Flags().GetBool("include-bots")
	includeDeleted, _ := cmd.Flags().GetBool("include-deleted")
	all, _ := cmd.Flags().GetBool("all")
	cursor, _ := cmd.Flags().GetString("cursor")
	cmdCtx, err := NewCommandContext(cmd, 15*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	service := usersops.NewService(cmdCtx.Client)
	result := &userSearchResult{OK: true, Query: query, Users: []usersops.UserInfo{}}
	seen := map[string]bool{}
	for {
		page, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*usersops.ListResult, error) {
			return service.List(cmdCtx.Ctx, usersops.ListParams{Limit: limit, Cursor: cursor, IncludeBots: true})
		})
		if err != nil {
			return err
		}
		for _, user := range page.Users {
			if !includeBots && user.IsBot {
				continue
			}
			if !includeDeleted && user.IsDeleted {
				continue
			}
			if userMatchesQuery(user, query) {
				result.Users = append(result.Users, user)
			}
		}
		result.NextCursor = page.NextCursor
		if page.NextCursor == "" {
			break
		}
		if !all {
			break
		}
		if seen[page.NextCursor] {
			return fmt.Errorf("Slack returned repeated users-list cursor %q", page.NextCursor)
		}
		seen[page.NextCursor] = true
		if err := appslack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.NextCursor
	}
	result.Matches = len(result.Users)
	return output.Print(cmd, result)
}

func userMatchesQuery(user usersops.UserInfo, query string) bool {
	query = strings.TrimSpace(query)
	if strings.EqualFold(user.UserID, query) || strings.EqualFold(user.ID, query) {
		return true
	}
	if strings.HasPrefix(query, "@") {
		username := strings.TrimSpace(strings.TrimPrefix(query, "@"))
		actual := strings.TrimPrefix(strings.TrimSpace(user.Username), "@")
		return username != "" && strings.EqualFold(actual, username)
	}
	if strings.Contains(query, "@") {
		return strings.EqualFold(strings.TrimSpace(user.Email), query)
	}
	for _, value := range []string{user.Name, user.Username, user.RealName, user.DisplayName} {
		if strings.Contains(strings.ToLower(value), strings.ToLower(query)) {
			return true
		}
	}
	return false
}

func runConversationsSearch(cmd *cobra.Command, _ []string) error {
	query, _ := cmd.Flags().GetString("query")
	nameFilter, _ := cmd.Flags().GetString("name")
	descriptionFilter, _ := cmd.Flags().GetString("description")
	query, nameFilter, descriptionFilter = strings.TrimSpace(query), strings.TrimSpace(nameFilter), strings.TrimSpace(descriptionFilter)
	if query == "" && nameFilter == "" && descriptionFilter == "" {
		return fmt.Errorf("provide --query, --name, or --description")
	}
	limit, _ := cmd.Flags().GetInt("limit")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	types, _ := cmd.Flags().GetStringSlice("types")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	all, _ := cmd.Flags().GetBool("all")
	cursor, _ := cmd.Flags().GetString("cursor")
	cmdCtx, err := NewCommandContext(cmd, 15*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	result := &conversationSearchResult{OK: true, Query: query, Name: nameFilter, Description: descriptionFilter, Conversations: []slack.Channel{}}
	seen := map[string]bool{}
	for {
		page, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (conversationPage, error) {
			channels, nextCursor, err := cmdCtx.Client.ListConversationsPage(cmdCtx.Ctx, appslack.ListChannelsParams{Limit: limit, Cursor: cursor, IncludeArchived: includeArchived, Types: types})
			return conversationPage{channels: channels, nextCursor: nextCursor}, err
		})
		if err != nil {
			return err
		}
		for _, channel := range page.channels {
			if conversationMatches(channel, query, nameFilter, descriptionFilter) {
				result.Conversations = append(result.Conversations, channel)
			}
		}
		result.NextCursor = page.nextCursor
		if page.nextCursor == "" {
			break
		}
		if !all {
			break
		}
		if seen[page.nextCursor] {
			return fmt.Errorf("Slack returned repeated conversations-list cursor %q", page.nextCursor)
		}
		seen[page.nextCursor] = true
		if err := appslack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.nextCursor
	}
	result.Matches = len(result.Conversations)
	return output.Print(cmd, result)
}

func conversationMatches(channel slack.Channel, query, nameFilter, descriptionFilter string) bool {
	name := strings.ToLower(channel.Name)
	description := strings.ToLower(channel.Purpose.Value + " " + channel.Topic.Value)
	if nameFilter != "" && !strings.Contains(name, strings.ToLower(nameFilter)) {
		return false
	}
	if descriptionFilter != "" && !strings.Contains(description, strings.ToLower(descriptionFilter)) {
		return false
	}
	if query == "" {
		return true
	}
	needle := strings.ToLower(query)
	return strings.Contains(name, needle) || strings.Contains(description, needle)
}
