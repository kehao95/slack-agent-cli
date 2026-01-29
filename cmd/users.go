package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/contentsquare/slack-cli/internal/config"
	"github.com/contentsquare/slack-cli/internal/output"
	"github.com/contentsquare/slack-cli/internal/slack"
	"github.com/contentsquare/slack-cli/internal/users"
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
	Long:  "List all workspace members with pagination support.",
	Example: `  slack-cli users list
  slack-cli users list --limit 50
  slack-cli users list --include-bots
  slack-cli users list --json`,
	RunE: runUsersList,
}

var usersInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get user details",
	Long:  "Get detailed information about a specific user.",
	Example: `  slack-cli users info --user U123ABC
  slack-cli users info --user @alice
  slack-cli users info --user U123ABC --json`,
	RunE: runUsersInfo,
}

var usersPresenceCmd = &cobra.Command{
	Use:   "presence",
	Short: "Check user presence",
	Long:  "Check the presence status of a specific user.",
	Example: `  slack-cli users presence --user U123ABC
  slack-cli users presence --user @alice
  slack-cli users presence --user U123ABC --json`,
	RunE: runUsersPresence,
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersInfoCmd)
	usersCmd.AddCommand(usersPresenceCmd)

	// users list flags
	usersListCmd.Flags().Int("limit", 100, "Maximum users per page")
	usersListCmd.Flags().String("cursor", "", "Continuation cursor for pagination")
	usersListCmd.Flags().Bool("include-bots", false, "Include bot users in results")

	// users info flags
	usersInfoCmd.Flags().String("user", "", "User ID or @username (required)")
	_ = usersInfoCmd.MarkFlagRequired("user")

	// users presence flags
	usersPresenceCmd.Flags().String("user", "", "User ID or @username (required)")
	_ = usersPresenceCmd.MarkFlagRequired("user")
}

func runUsersList(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	client := slack.New(cfg.BotToken)
	service := users.NewService(client)

	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	includeBots, _ := cmd.Flags().GetBool("include-bots")

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	result, err := service.List(ctx, users.ListParams{
		Limit:       limit,
		Cursor:      cursor,
		IncludeBots: includeBots,
	})
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}

func runUsersInfo(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	client := slack.New(cfg.BotToken)
	service := users.NewService(client)

	userInput, _ := cmd.Flags().GetString("user")
	if userInput == "" {
		return fmt.Errorf("--user flag is required")
	}

	// Resolve user ID from @username or user ID
	userID, err := resolveUserID(cmd.Context(), client, userInput)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	result, err := service.GetInfo(ctx, userID)
	if err != nil {
		return err
	}

	return output.Print(cmd, result)
}

func runUsersPresence(cmd *cobra.Command, args []string) error {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}

	userInput, _ := cmd.Flags().GetString("user")
	if userInput == "" {
		return fmt.Errorf("--user flag is required")
	}

	// For now, return a not implemented message
	// The Slack SDK supports presence via GetUserPresence, but we'll implement
	// this later as it requires additional API permissions
	return fmt.Errorf("users presence command not yet implemented - requires additional API scopes")
}

// resolveUserID converts @username to user ID, or returns the input if it's already an ID.
func resolveUserID(ctx context.Context, client *slack.APIClient, input string) (string, error) {
	// If it starts with @, try to resolve as username
	if strings.HasPrefix(input, "@") {
		username := strings.TrimPrefix(input, "@")
		// We need to list users and find by name
		allUsers, _, err := client.ListUsers(ctx, "", 1000)
		if err != nil {
			return "", fmt.Errorf("list users to resolve name: %w", err)
		}
		for _, u := range allUsers {
			if u.Name == username || u.Profile.DisplayName == username {
				return u.ID, nil
			}
		}
		return "", fmt.Errorf("user @%s not found", username)
	}

	// Assume it's already a user ID
	return input, nil
}
