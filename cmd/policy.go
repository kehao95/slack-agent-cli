package cmd

import (
	"fmt"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/policy"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

// Every runnable command has a reviewed operation. Empty entries are local
// operations; methods describe the command's remote effect, not its HTTP verb.
// SDK/raw calls enforce the same method policy separately.
var commandOperations = map[string]string{
	"": "", "help": "", "completion": "",
	"completion bash": "", "completion zsh": "", "completion fish": "", "completion powershell": "",
	"auth login": "auth.test", "auth test": "auth.test", "auth whoami": "auth.test",
	"auth oauth":     "oauth.v2.access",
	"cache populate": "conversations.list", "cache status": "", "cache clear": "",
	"channels list": "conversations.list", "channels info": "conversations.info",
	"channels join": "conversations.join", "channels leave": "conversations.leave",
	"conversations list": "conversations.list", "conversations info": "conversations.info",
	"conversations members": "conversations.members", "conversations create": "conversations.create",
	"conversations archive": "conversations.archive", "conversations unarchive": "conversations.unarchive",
	"conversations rename": "conversations.rename", "conversations topic": "conversations.setTopic",
	"conversations purpose": "conversations.setPurpose", "conversations invite": "conversations.invite",
	"conversations kick": "conversations.kick", "conversations open": "conversations.open",
	"conversations close": "conversations.close", "conversations mark": "conversations.mark",
	"conversations join": "conversations.join", "conversations leave": "conversations.leave",
	"messages list": "conversations.history", "messages search": "search.messages", "messages next": "",
	"messages send": "chat.postMessage", "messages edit": "chat.update", "messages delete": "chat.delete",
	"messages permalink": "chat.getPermalink", "messages ephemeral": "chat.postEphemeral",
	"messages schedule": "chat.scheduleMessage", "messages scheduled list": "chat.scheduledMessages.list",
	"messages scheduled delete": "chat.deleteScheduledMessage", "messages stream start": "chat.startStream",
	"messages stream append": "chat.appendStream", "messages stream stop": "chat.stopStream",
	"files upload": "files.getUploadURLExternal", "files download": "files.info", "files list": "files.list",
	"files info": "files.info", "files delete": "files.delete", "files share-public": "files.sharedPublicURL",
	"files revoke-public": "files.revokePublicURL",
	"search all":          "search.all", "search messages": "search.messages", "search files": "search.files",
	"events stream": "apps.connections.open", "events list": "", "events get": "", "events next": "",
	"events claim": "", "events ack": "", "daemon run": "apps.connections.open", "daemon status": "",
	"lists items": "slackLists.items.list", "lists item": "slackLists.items.info",
	"reactions add": "reactions.add", "reactions remove": "reactions.remove", "reactions list": "reactions.get",
	"pins add": "pins.add", "pins remove": "pins.remove", "pins list": "pins.list",
	"users list": "users.list", "users info": "users.info", "users lookup": "users.lookupByEmail",
	"users profile": "users.profile.get", "users presence": "users.getPresence",
	"users status get": "users.profile.get", "users status set": "users.profile.set",
	"users status clear": "users.profile.set", "users conversations": "users.conversations",
	"usergroups list": "usergroups.list", "usergroups members": "usergroups.users.list",
	"usergroups members set": "usergroups.users.update", "usergroups create": "usergroups.create",
	"usergroups update": "usergroups.update", "usergroups enable": "usergroups.enable",
	"usergroups disable": "usergroups.disable", "emoji list": "emoji.list",
	"users search": "users.list", "conversations search": "conversations.list",
	"users profile set":  "users.profile.set",
	"users presence get": "users.getPresence", "users presence set": "users.setPresence",
	"users photo set": "users.setPhoto", "users photo delete": "users.deletePhoto",
	"users dnd info": "dnd.info", "users dnd team": "dnd.teamInfo",
	"users dnd snooze": "dnd.setSnooze", "users dnd end": "dnd.endDnd",
	"usergroups members add": "usergroups.users.update", "usergroups members remove": "usergroups.users.update",
	"messages get": "conversations.history", "messages preview": "",
	"files read": "files.info", "files export": "files.info",
	"bookmarks list": "bookmarks.list", "bookmarks add": "bookmarks.add",
	"bookmarks edit": "bookmarks.edit", "bookmarks remove": "bookmarks.remove",
	"canvases list": "files.list", "canvases read": "files.info", "canvases export": "files.info",
	"canvases create": "canvases.create", "canvases channel-create": "conversations.canvases.create",
	"canvases edit": "canvases.edit", "canvases delete": "canvases.delete",
	"canvases sections": "canvases.sections.lookup", "canvases share": "canvases.access.set",
	"canvases revoke": "canvases.access.delete",
	"lists create":    "slackLists.create", "lists update": "slackLists.update",
	"lists item-add": "slackLists.items.create", "lists item-update": "slackLists.items.update",
	"lists item-delete": "slackLists.items.delete", "lists items-delete": "slackLists.items.deleteMultiple",
	"lists share": "slackLists.access.set", "lists revoke": "slackLists.access.delete",
	"lists export": "slackLists.download.start", "lists export-start": "slackLists.download.start",
	"lists export-get": "slackLists.download.get",
}

func commandOperationKey(cmd *cobra.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()))
}

// enforceCommandPolicy runs before handlers, config/auth discovery, cache
// refreshes, stdin reads, and OAuth listeners. Aliases resolve to canonical paths.
func enforceCommandPolicy(cmd *cobra.Command, args []string) error {
	readOnly, err := policy.ReadOnly()
	if err != nil || !readOnly {
		return err
	}
	key := commandOperationKey(cmd)
	if key == "api" {
		if len(args) != 1 {
			return fmt.Errorf("a Slack API method is required")
		}
		method := strings.TrimSpace(args[0])
		if err := slack.ValidateAPIMethod(method); err != nil {
			return err
		}
		if err := policy.CheckMethod(method); err != nil {
			return err
		}
		return requireOrdinarySearchUser(method)
	}
	method, reviewed := commandOperations[key]
	if !reviewed {
		return policy.CheckMethod("unreviewed command: " + key)
	}
	if method != "" {
		if err := policy.CheckMethod(method); err != nil {
			return err
		}
	}
	// ResolveChannel uses conversations.open for @handles even on read commands.
	// Refuse before auth discovery or username enumeration; existing D IDs work.
	if channel, _ := cmd.Flags().GetString("channel"); strings.HasPrefix(strings.TrimSpace(channel), "@") {
		return fmt.Errorf("%w; use an existing DM conversation ID (D...) instead of --channel @username", policy.CheckMethod("conversations.open"))
	}
	return nil
}
