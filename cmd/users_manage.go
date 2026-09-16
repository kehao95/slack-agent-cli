package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/output"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var usersProfileSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update selected profile fields",
	Long: `Update only explicitly supplied profile fields with users.profile.set.

Use --field name=value for standard or future Slack profile keys. Use
--custom-field FIELD_ID=value for one custom field, or --custom-fields with a
JSON object. The profile object is sent as a partial update; omitted fields are
not read or rewritten. Updating another user requires Slack admin privileges.`,
	Example: `  slk users profile set --field title=Engineer --field pronouns=they/them
  slk users profile set --custom-field Xf0123=Coffee --user U123ABC
  slk users profile set --custom-fields '{"Xf0123":{"value":"Coffee","alt":""}}'`,
	RunE: runUsersProfileSet,
}

var usersPresenceSetCmd = &cobra.Command{
	Use:     "set",
	Short:   "Set the authenticated user's presence",
	Long:    "Set manual presence to auto or away using users.setPresence.",
	Example: "  slk users presence set --presence away\n  slk users presence set --presence auto",
	RunE:    runUsersPresenceSet,
}

var usersPresenceGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Check user presence",
	RunE:  runUsersPresence,
}

var usersPhotoCmd = &cobra.Command{Use: "photo", Short: "Manage the authenticated user's profile photo"}
var usersPhotoSetCmd = &cobra.Command{
	Use:     "set",
	Short:   "Set the authenticated user's profile photo",
	Example: "  slk users photo set --file ./avatar.png",
	RunE:    runUsersPhotoSet,
}
var usersPhotoDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the authenticated user's profile photo",
	RunE:  runUsersPhotoDelete,
}

var usersDNDCmd = &cobra.Command{Use: "dnd", Short: "Manage Do Not Disturb"}
var usersDNDInfoCmd = &cobra.Command{Use: "info", Short: "Get Do Not Disturb status", RunE: runUsersDNDInfo}
var usersDNDTeamCmd = &cobra.Command{Use: "team", Short: "Get Do Not Disturb status for users", RunE: runUsersDNDTeam}
var usersDNDSnoozeCmd = &cobra.Command{Use: "snooze", Short: "Start a Do Not Disturb snooze", RunE: runUsersDNDSnooze}
var usersDNDEndCmd = &cobra.Command{Use: "end", Short: "End Do Not Disturb or snooze", RunE: runUsersDNDEnd}

type dndMutationResult struct {
	OK     bool                `json:"ok"`
	Action string              `json:"action"`
	Status *slackapi.DNDStatus `json:"status,omitempty"`
}

func init() {
	usersProfileCmd.AddCommand(usersProfileSetCmd)
	usersPresenceCmd.AddCommand(usersPresenceGetCmd, usersPresenceSetCmd)
	usersCmd.AddCommand(usersPhotoCmd, usersDNDCmd)
	usersPhotoCmd.AddCommand(usersPhotoSetCmd, usersPhotoDeleteCmd)
	usersDNDCmd.AddCommand(usersDNDInfoCmd, usersDNDTeamCmd, usersDNDSnoozeCmd, usersDNDEndCmd)

	usersProfileSetCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	usersProfileSetCmd.Flags().StringArray("field", nil, "Profile field assignment name=value (repeatable)")
	usersProfileSetCmd.Flags().StringArray("custom-field", nil, "Custom profile field assignment FIELD_ID=value or FIELD_ID=value|alt (repeatable)")
	usersProfileSetCmd.Flags().String("custom-fields", "", "Custom profile fields as a JSON object")
	usersProfileSetCmd.Flags().String("profile", "", "Additional partial profile fields as a JSON object or @file")

	usersPresenceSetCmd.Flags().String("presence", "", "Presence value: auto or away (required)")
	_ = usersPresenceSetCmd.MarkFlagRequired("presence")
	usersPresenceGetCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (required)")
	_ = usersPresenceGetCmd.MarkFlagRequired("user")

	usersPhotoSetCmd.Flags().String("file", "", "Image file path (required)")
	_ = usersPhotoSetCmd.MarkFlagRequired("file")
	usersPhotoSetCmd.Flags().Int("crop-x", slackapi.DEFAULT_USER_PHOTO_CROP_X, "Photo crop origin X")
	usersPhotoSetCmd.Flags().Int("crop-y", slackapi.DEFAULT_USER_PHOTO_CROP_Y, "Photo crop origin Y")
	usersPhotoSetCmd.Flags().Int("crop-w", slackapi.DEFAULT_USER_PHOTO_CROP_W, "Photo crop width")

	usersDNDInfoCmd.Flags().String("user", "", "Canonical user ID, <@ID>, or @username (default: authenticated user)")
	usersDNDInfoCmd.Flags().String("team-id", "", "Team ID for org-level credentials (default: active workspace)")
	usersDNDTeamCmd.Flags().StringSlice("users", nil, "Comma-separated canonical user IDs, <@ID> mentions, or @usernames (required)")
	usersDNDTeamCmd.Flags().String("team-id", "", "Team ID for org-level credentials (default: active workspace)")
	_ = usersDNDTeamCmd.MarkFlagRequired("users")
	usersDNDSnoozeCmd.Flags().Int("minutes", 0, "Snooze duration in minutes (required)")
	_ = usersDNDSnoozeCmd.MarkFlagRequired("minutes")
	usersDNDEndCmd.Flags().Bool("snooze", false, "End the current snooze instead of the scheduled DND session")
}

func runUsersProfileSet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	profile, err := profileUpdatePayload(cmd)
	if err != nil {
		return err
	}
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	result, err := cmdCtx.Client.SetUserProfileFields(cmdCtx.Ctx, userID, profile)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func profileUpdatePayload(cmd *cobra.Command) (map[string]interface{}, error) {
	profile := make(map[string]interface{})
	assignments, _ := cmd.Flags().GetStringArray("field")
	for _, assignment := range assignments {
		key, value, err := splitAssignment(assignment)
		if err != nil {
			return nil, fmt.Errorf("parse --field: %w", err)
		}
		profile[key] = value
	}
	customAssignments, _ := cmd.Flags().GetStringArray("custom-field")
	custom := make(map[string]slackapi.UserProfileCustomField)
	for _, assignment := range customAssignments {
		key, value, err := splitAssignment(assignment)
		if err != nil {
			return nil, fmt.Errorf("parse --custom-field: %w", err)
		}
		parts := strings.SplitN(value, "|", 2)
		field := slackapi.UserProfileCustomField{Value: parts[0]}
		if len(parts) == 2 {
			field.Alt = parts[1]
		}
		custom[key] = field
	}
	customJSON, _ := cmd.Flags().GetString("custom-fields")
	if strings.TrimSpace(customJSON) != "" {
		var parsed map[string]slackapi.UserProfileCustomField
		if err := json.Unmarshal([]byte(customJSON), &parsed); err != nil {
			return nil, fmt.Errorf("parse --custom-fields as JSON object: %w", err)
		}
		for key, value := range parsed {
			custom[key] = value
		}
	}
	if len(custom) > 0 {
		profile["fields"] = custom
	}
	profileJSON, _ := cmd.Flags().GetString("profile")
	if strings.TrimSpace(profileJSON) != "" {
		parsed, err := readAPIData(cmd, profileJSON)
		if err != nil {
			return nil, fmt.Errorf("parse --profile: %w", err)
		}
		for key, value := range parsed {
			if key == "fields" && len(custom) > 0 {
				continue
			}
			profile[key] = value
		}
	}
	if len(profile) == 0 {
		return nil, fmt.Errorf("provide at least one --field, --custom-field, --custom-fields, or --profile")
	}
	return profile, nil
}

func splitAssignment(value string) (string, string, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("expected name=value, got %q", value)
	}
	return strings.TrimSpace(parts[0]), parts[1], nil
}

func runUsersPresenceSet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	presence, _ := cmd.Flags().GetString("presence")
	if err := cmdCtx.Client.SetUserPresence(cmdCtx.Ctx, presence); err != nil {
		return err
	}
	return output.Print(cmd, map[string]interface{}{"ok": true, "presence": strings.ToLower(strings.TrimSpace(presence))})
}

func runUsersPhotoSet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	file, _ := cmd.Flags().GetString("file")
	params := slackapi.NewUserSetPhotoParams()
	params.CropX, _ = cmd.Flags().GetInt("crop-x")
	params.CropY, _ = cmd.Flags().GetInt("crop-y")
	params.CropW, _ = cmd.Flags().GetInt("crop-w")
	if err := cmdCtx.Client.SetUserPhoto(cmdCtx.Ctx, file, params); err != nil {
		return err
	}
	return output.Print(cmd, map[string]interface{}{"ok": true, "action": "set-photo"})
}

func runUsersPhotoDelete(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	if err := cmdCtx.Client.DeleteUserPhoto(cmdCtx.Ctx); err != nil {
		return err
	}
	return output.Print(cmd, map[string]interface{}{"ok": true, "action": "delete-photo"})
}

func runUsersDNDInfo(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	userID, err := optionalResolvedUser(cmdCtx, cmd)
	if err != nil {
		return err
	}
	teamID, _ := cmd.Flags().GetString("team-id")
	status, err := cmdCtx.Client.GetDNDInfo(cmdCtx.Ctx, userID, teamID)
	if err != nil {
		return err
	}
	return output.Print(cmd, status)
}

func runUsersDNDTeam(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	refs, _ := cmd.Flags().GetStringSlice("users")
	userIDs, err := resolveUserReferences(cmdCtx, refs)
	if err != nil {
		return fmt.Errorf("resolve users: %w", err)
	}
	teamID, _ := cmd.Flags().GetString("team-id")
	status, err := cmdCtx.Client.GetDNDTeamInfo(cmdCtx.Ctx, userIDs, teamID)
	if err != nil {
		return err
	}
	return output.Print(cmd, map[string]interface{}{"ok": true, "users": status})
}

func runUsersDNDSnooze(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	minutes, _ := cmd.Flags().GetInt("minutes")
	status, err := cmdCtx.Client.SetDNDSnooze(cmdCtx.Ctx, minutes)
	if err != nil {
		return err
	}
	return output.Print(cmd, dndMutationResult{OK: true, Action: "snooze", Status: status})
}

func runUsersDNDEnd(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	snooze, _ := cmd.Flags().GetBool("snooze")
	if snooze {
		status, err := cmdCtx.Client.EndDNDSnooze(cmdCtx.Ctx)
		if err != nil {
			return err
		}
		return output.Print(cmd, dndMutationResult{OK: true, Action: "end-snooze", Status: status})
	}
	if err := cmdCtx.Client.EndDND(cmdCtx.Ctx); err != nil {
		return err
	}
	return output.Print(cmd, dndMutationResult{OK: true, Action: "end-dnd"})
}
