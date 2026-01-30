package cmd

import (
	"context"
	"fmt"
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
        "id": "U123ABC",
        "name": "alice",
        "real_name": "Alice Smith",
        "display_name": "alice",
        "is_bot": false,
        "is_deleted": false,
        "profile": {
          "email": "alice@example.com",
          "status_text": "In a meeting",
          "status_emoji": ":calendar:"
        }
      }
    ]
  }

Note: Set --include-bots to include bot users in results.`,
	Example: `  # List all users
  slack-agent-cli users list

  # List with pagination
  slack-agent-cli users list --limit 50 --cursor "dXNlcl9pZDo..."

  # Include bot users
  slack-agent-cli users list --include-bots`,
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
      "id": "U123ABC",
      "name": "alice",
      "real_name": "Alice Smith",
      "display_name": "alice",
      "is_bot": false,
      "is_deleted": false,
      "profile": {
        "email": "alice@example.com",
        "phone": "+1234567890",
        "title": "Engineer",
        "status_text": "In a meeting",
        "status_emoji": ":calendar:",
        "avatar_hash": "abc123"
      },
      "tz": "America/New_York",
      "tz_label": "Eastern Standard Time"
    }
  }

User Identifier:
  - User ID: U123ABC (direct lookup)
  - Username: @alice (resolved via user list)`,
	Example: `  # Get user info by ID
  slack-agent-cli users info --user U123ABC

  # Get user info by username
  slack-agent-cli users info --user @alice`,
	RunE: runUsersInfo,
}

var usersPresenceCmd = &cobra.Command{
	Use:   "presence",
	Short: "Check user presence",
	Long:  "Check the presence status of a specific user.",
	Example: `  slack-agent-cli users presence --user U123ABC
  slack-agent-cli users presence --user @alice
  slack-agent-cli users presence --user U123ABC --human`,
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
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	service := users.NewService(cmdCtx.Client)

	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	includeBots, _ := cmd.Flags().GetBool("include-bots")

	result, err := service.List(cmdCtx.Ctx, users.ListParams{
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
	userID, err := resolveUserID(cmd.Context(), cmdCtx.Client, userInput)
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
