package cmd

import (
	"fmt"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var bookmarksCmd = &cobra.Command{Use: "bookmarks", Short: "Manage channel bookmarks"}

var bookmarksListCmd = &cobra.Command{Use: "list", Short: "List bookmarks in a channel", RunE: runBookmarksList}
var bookmarksAddCmd = &cobra.Command{Use: "add", Short: "Add a bookmark to a channel", RunE: runBookmarksAdd}
var bookmarksEditCmd = &cobra.Command{Use: "edit", Short: "Edit a channel bookmark", RunE: runBookmarksEdit}
var bookmarksRemoveCmd = &cobra.Command{Use: "remove", Short: "Remove a channel bookmark", RunE: runBookmarksRemove}

func init() {
	rootCmd.AddCommand(bookmarksCmd)
	bookmarksCmd.AddCommand(bookmarksListCmd, bookmarksAddCmd, bookmarksEditCmd, bookmarksRemoveCmd)

	bookmarksListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	bookmarksAddCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	bookmarksAddCmd.Flags().String("title", "", "Bookmark title (required)")
	bookmarksAddCmd.Flags().String("type", "link", "Bookmark type (currently only link is publicly supported)")
	bookmarksAddCmd.Flags().String("link", "", "Bookmark URL (required for link bookmarks)")
	bookmarksAddCmd.Flags().String("emoji", "", "Emoji name for the bookmark")
	bookmarksAddCmd.Flags().String("entity-id", "", "Slack entity ID for an internal bookmark")
	bookmarksAddCmd.Flags().String("parent-id", "", "Parent bookmark ID")
	bookmarksAddCmd.Flags().String("access-level", "", "Bookmark access level: read or write")

	bookmarksEditCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	bookmarksEditCmd.Flags().String("bookmark", "", "Bookmark ID (required)")
	bookmarksEditCmd.Flags().String("title", "", "Replacement bookmark title")
	bookmarksEditCmd.Flags().String("link", "", "Replacement bookmark URL")
	bookmarksEditCmd.Flags().String("emoji", "", "Replacement emoji name")

	bookmarksRemoveCmd.Flags().StringP("channel", "c", "", "Channel name or ID")
	bookmarksRemoveCmd.Flags().String("bookmark", "", "Bookmark ID")
	bookmarksRemoveCmd.Flags().String("section-id", "", "Quip section ID to unbookmark")
}

func bookmarkChannelID(cmd *cobra.Command, ctx *CommandContext, required bool) (string, error) {
	value, _ := cmd.Flags().GetString("channel")
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("--channel is required")
		}
		return "", nil
	}
	return ctx.ResolveChannel(value)
}

func runBookmarksList(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channelID, err := bookmarkChannelID(cmd, ctx, true)
	if err != nil {
		return err
	}
	result, err := ctx.Client.ListBookmarks(ctx.Ctx, channelID)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runBookmarksAdd(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channelID, err := bookmarkChannelID(cmd, ctx, true)
	if err != nil {
		return err
	}
	title, _ := cmd.Flags().GetString("title")
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("--title is required")
	}
	typeName, _ := cmd.Flags().GetString("type")
	typeName = strings.TrimSpace(typeName)
	if typeName != "link" {
		return fmt.Errorf("--type %q is not supported by Slack's public bookmarks.add API; use link", typeName)
	}
	link, _ := cmd.Flags().GetString("link")
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Errorf("--link is required for link bookmarks")
	}
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		return fmt.Errorf("--link must begin with http:// or https://")
	}
	access, _ := cmd.Flags().GetString("access-level")
	if access != "" && access != "read" && access != "write" {
		return fmt.Errorf("--access-level must be read or write")
	}
	result, err := ctx.Client.AddBookmark(ctx.Ctx, appslack.BookmarkParams{
		ChannelID: channelID, Title: title, Type: typeName, Link: link,
		Emoji: artifactFlagString(cmd, "emoji"), EntityID: artifactFlagString(cmd, "entity-id"),
		ParentID: artifactFlagString(cmd, "parent-id"), AccessLevel: access,
	})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runBookmarksEdit(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channelID, err := bookmarkChannelID(cmd, ctx, true)
	if err != nil {
		return err
	}
	bookmarkID := artifactFlagString(cmd, "bookmark")
	if bookmarkID == "" {
		return fmt.Errorf("--bookmark is required")
	}
	params := appslack.BookmarkParams{ChannelID: channelID, BookmarkID: bookmarkID}
	for _, name := range []string{"title", "link", "emoji"} {
		value := artifactFlagString(cmd, name)
		if value != "" {
			switch name {
			case "title":
				params.Title = value
			case "link":
				if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
					return fmt.Errorf("--link must begin with http:// or https://")
				}
				params.Link = value
			case "emoji":
				params.Emoji = value
			}
		}
	}
	if params.Title == "" && params.Link == "" && params.Emoji == "" {
		return fmt.Errorf("at least one of --title, --link, or --emoji is required")
	}
	result, err := ctx.Client.EditBookmark(ctx.Ctx, params)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runBookmarksRemove(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channel, _ := cmd.Flags().GetString("channel")
	bookmarkID := artifactFlagString(cmd, "bookmark")
	sectionID := artifactFlagString(cmd, "section-id")
	if sectionID == "" && (strings.TrimSpace(channel) == "" || bookmarkID == "") {
		return fmt.Errorf("--channel and --bookmark are required unless --section-id is provided")
	}
	if sectionID != "" && (strings.TrimSpace(channel) != "" || bookmarkID != "") {
		return fmt.Errorf("--section-id cannot be combined with --channel or --bookmark")
	}
	channelID := ""
	if strings.TrimSpace(channel) != "" {
		channelID, err = ctx.ResolveChannel(channel)
		if err != nil {
			return err
		}
	}
	result, err := ctx.Client.RemoveBookmark(ctx.Ctx, channelID, bookmarkID, sectionID)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func artifactFlagString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return strings.TrimSpace(value)
}

func printArtifactResponse(cmd *cobra.Command, result appslack.ArtifactResponse) error {
	return output.Print(cmd, result)
}
