package cmd

import (
	"fmt"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	usergroupops "github.com/kehao95/slack-agent-cli/internal/usergroups"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var usergroupsCmd = &cobra.Command{Use: "usergroups", Aliases: []string{"user-groups"}, Short: "User group operations", Long: "List and manage Slack user groups and their membership."}
var usergroupsListCmd = &cobra.Command{Use: "list", Short: "List user groups", Example: "  slk usergroups list --include-users --include-disabled", RunE: runUsergroupsList}
var usergroupsMembersCmd = &cobra.Command{Use: "members", Short: "List user group members", Example: "  slk usergroups members --group @engineering", RunE: runUsergroupsMembers}
var usergroupsMembersSetCmd = &cobra.Command{Use: "set", Short: "Replace user group members", Example: "  slk usergroups members set --group @engineering --members @alice,@bob", RunE: runUsergroupsMembersSet}
var usergroupsMembersAddCmd = &cobra.Command{Use: "add", Short: "Add users to a user group", Example: "  slk usergroups members add --group @engineering --members @alice,@bob", RunE: runUsergroupsMembersAdd}
var usergroupsMembersRemoveCmd = &cobra.Command{Use: "remove", Short: "Remove users from a user group", Example: "  slk usergroups members remove --group @engineering --members @alice,@bob", RunE: runUsergroupsMembersRemove}
var usergroupsCreateCmd = &cobra.Command{Use: "create", Short: "Create a user group", Example: "  slk usergroups create --name Engineering --handle engineering --description 'Engineering team'", RunE: runUsergroupsCreate}
var usergroupsUpdateCmd = &cobra.Command{Use: "update", Short: "Update a user group", Example: "  slk usergroups update --group @engineering --description 'Product engineering'", RunE: runUsergroupsUpdate}
var usergroupsEnableCmd = &cobra.Command{Use: "enable", Short: "Enable a user group", Example: "  slk usergroups enable --group @engineering", RunE: func(cmd *cobra.Command, _ []string) error { return runUsergroupsEnabled(cmd, true) }}
var usergroupsDisableCmd = &cobra.Command{Use: "disable", Short: "Disable a user group", Example: "  slk usergroups disable --group @engineering", RunE: func(cmd *cobra.Command, _ []string) error { return runUsergroupsEnabled(cmd, false) }}

func init() {
	rootCmd.AddCommand(usergroupsCmd)
	usergroupsCmd.AddCommand(usergroupsListCmd, usergroupsMembersCmd, usergroupsCreateCmd, usergroupsUpdateCmd, usergroupsEnableCmd, usergroupsDisableCmd)
	usergroupsMembersCmd.AddCommand(usergroupsMembersSetCmd, usergroupsMembersAddCmd, usergroupsMembersRemoveCmd)
	usergroupsListCmd.Flags().Bool("include-users", false, "Include member IDs")
	usergroupsListCmd.Flags().Bool("include-count", true, "Include member counts")
	usergroupsListCmd.Flags().Bool("include-disabled", false, "Include disabled user groups")

	usergroupsMembersCmd.Flags().String("group", "", "User group ID, @handle, or name (required)")
	_ = usergroupsMembersCmd.MarkFlagRequired("group")
	usergroupsMembersSetCmd.Flags().String("group", "", "User group ID, @handle, or name (required)")
	usergroupsMembersSetCmd.Flags().String("members", "", "Comma-separated canonical user IDs, <@ID> mentions, or @usernames (required)")
	_ = usergroupsMembersSetCmd.MarkFlagRequired("group")
	_ = usergroupsMembersSetCmd.MarkFlagRequired("members")
	for _, command := range []*cobra.Command{usergroupsMembersAddCmd, usergroupsMembersRemoveCmd} {
		command.Flags().String("group", "", "User group ID, @handle, or name (required)")
		command.Flags().String("members", "", "Comma-separated canonical user IDs, <@ID> mentions, or @usernames (required)")
		_ = command.MarkFlagRequired("group")
		_ = command.MarkFlagRequired("members")
	}

	usergroupsCreateCmd.Flags().String("name", "", "Display name (required)")
	usergroupsCreateCmd.Flags().String("handle", "", "Mention handle")
	usergroupsCreateCmd.Flags().String("description", "", "Description")
	usergroupsCreateCmd.Flags().String("channels", "", "Comma-separated default channels")
	usergroupsCreateCmd.Flags().Bool("enable-section", false, "Enable the user group sidebar section")
	usergroupsCreateCmd.Flags().Bool("include-count", true, "Include member count in response")
	_ = usergroupsCreateCmd.MarkFlagRequired("name")

	usergroupsUpdateCmd.Flags().String("group", "", "User group ID, @handle, or name (required)")
	usergroupsUpdateCmd.Flags().String("name", "", "New display name")
	usergroupsUpdateCmd.Flags().String("handle", "", "New mention handle")
	usergroupsUpdateCmd.Flags().String("description", "", "New description; pass an empty value to clear")
	usergroupsUpdateCmd.Flags().String("channels", "", "Comma-separated default channels; pass an empty value to clear")
	usergroupsUpdateCmd.Flags().Bool("enable-section", false, "Enable the user group sidebar section")
	_ = usergroupsUpdateCmd.MarkFlagRequired("group")
	for _, command := range []*cobra.Command{usergroupsEnableCmd, usergroupsDisableCmd} {
		command.Flags().String("group", "", "User group ID, @handle, or name (required)")
		command.Flags().Bool("include-count", true, "Include member count in response")
		_ = command.MarkFlagRequired("group")
	}
}

func runUsergroupsList(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	includeUsers, _ := cmd.Flags().GetBool("include-users")
	includeCount, _ := cmd.Flags().GetBool("include-count")
	includeDisabled, _ := cmd.Flags().GetBool("include-disabled")
	result, err := usergroupops.NewService(cmdCtx.Client).List(cmdCtx.Ctx, appslack.UserGroupListOptions{IncludeUsers: includeUsers, IncludeCount: includeCount, IncludeDisabled: includeDisabled, TeamID: cmdCtx.TeamID})
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsMembers(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	group, _ := cmd.Flags().GetString("group")
	result, err := usergroupops.NewService(cmdCtx.Client).Members(cmdCtx.Ctx, group)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsMembersSet(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	group, _ := cmd.Flags().GetString("group")
	value, _ := cmd.Flags().GetString("members")
	members, err := resolveUserReferences(cmdCtx, splitNonEmpty(value))
	if err != nil {
		return fmt.Errorf("resolve members: %w", err)
	}
	result, err := usergroupops.NewService(cmdCtx.Client).SetMembers(cmdCtx.Ctx, group, members)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsMembersAdd(cmd *cobra.Command, _ []string) error {
	return runUsergroupsMembersDelta(cmd, true)
}

func runUsergroupsMembersRemove(cmd *cobra.Command, _ []string) error {
	return runUsergroupsMembersDelta(cmd, false)
}

func runUsergroupsMembersDelta(cmd *cobra.Command, add bool) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	group, _ := cmd.Flags().GetString("group")
	value, _ := cmd.Flags().GetString("members")
	members, err := resolveUserReferences(cmdCtx, splitNonEmpty(value))
	if err != nil {
		return fmt.Errorf("resolve members: %w", err)
	}
	service := usergroupops.NewService(cmdCtx.Client)
	var result *usergroupops.MutationResult
	if add {
		result, err = service.AddMembers(cmdCtx.Ctx, group, members)
	} else {
		result, err = service.RemoveMembers(cmdCtx.Ctx, group, members)
	}
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsCreate(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	name, _ := cmd.Flags().GetString("name")
	handle, _ := cmd.Flags().GetString("handle")
	description, _ := cmd.Flags().GetString("description")
	channels, err := resolveChannelList(cmdCtx, flagString(cmd, "channels"))
	if err != nil {
		return err
	}
	enableSection, _ := cmd.Flags().GetBool("enable-section")
	includeCount, _ := cmd.Flags().GetBool("include-count")
	group := slackapi.UserGroup{Name: name, Handle: strings.TrimPrefix(handle, "@"), Description: description, TeamID: cmdCtx.TeamID, Prefs: slackapi.UserGroupPrefs{Channels: channels}}
	result, err := usergroupops.NewService(cmdCtx.Client).Create(cmdCtx.Ctx, group, includeCount, enableSection)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsUpdate(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("handle") && !cmd.Flags().Changed("description") && !cmd.Flags().Changed("channels") && !cmd.Flags().Changed("enable-section") {
		return fmt.Errorf("provide at least one field to update")
	}
	group, _ := cmd.Flags().GetString("group")
	opts := appslack.UserGroupUpdateOptions{TeamID: cmdCtx.TeamID}
	if cmd.Flags().Changed("name") {
		opts.Name = flagString(cmd, "name")
	}
	if cmd.Flags().Changed("handle") {
		opts.Handle = strings.TrimPrefix(flagString(cmd, "handle"), "@")
	}
	if cmd.Flags().Changed("description") {
		value := flagString(cmd, "description")
		opts.Description = &value
	}
	if cmd.Flags().Changed("channels") {
		channels, err := resolveChannelList(cmdCtx, flagString(cmd, "channels"))
		if err != nil {
			return err
		}
		opts.Channels = &channels
	}
	if cmd.Flags().Changed("enable-section") {
		opts.EnableSection, _ = cmd.Flags().GetBool("enable-section")
		if !opts.EnableSection {
			return fmt.Errorf("the Slack SDK can enable a user group section but cannot disable it; omit --enable-section=false")
		}
	}
	result, err := usergroupops.NewService(cmdCtx.Client).Update(cmdCtx.Ctx, group, opts)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func runUsergroupsEnabled(cmd *cobra.Command, enabled bool) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	group, _ := cmd.Flags().GetString("group")
	includeCount, _ := cmd.Flags().GetBool("include-count")
	result, err := usergroupops.NewService(cmdCtx.Client).SetEnabled(cmdCtx.Ctx, group, enabled, includeCount, cmdCtx.TeamID)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}

func resolveChannelList(cmdCtx *CommandContext, value string) ([]string, error) {
	channels := splitNonEmpty(value)
	for i, channel := range channels {
		id, err := cmdCtx.ResolveChannel(channel)
		if err != nil {
			return nil, fmt.Errorf("resolve channel %q: %w", channel, err)
		}
		channels[i] = id
	}
	return channels, nil
}
func flagString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}
