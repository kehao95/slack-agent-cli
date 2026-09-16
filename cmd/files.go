package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	fileops "github.com/kehao95/slack-agent-cli/internal/files"
	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var filesCmd = &cobra.Command{Use: "files", Short: "File operations", Long: "Upload, download, list, inspect, delete, and manage public links for Slack files."}

var filesUploadCmd = &cobra.Command{
	Use: "upload", Short: "Upload a local file",
	Example: "  slk files upload --file ./report.pdf --channel '#general' --title 'Weekly report'\n  slk files upload --file ./trace.txt --snippet-type text",
	RunE:    runFilesUpload,
}
var filesDownloadCmd = &cobra.Command{
	Use: "download", Short: "Download a Slack file",
	Example: "  slk files download --file F123 --output ./report.pdf\n  slk files download --file F123 --output ./report.pdf --force",
	RunE:    runFilesDownload,
}
var filesListCmd = &cobra.Command{
	Use: "list", Short: "List Slack files",
	Example: "  slk files list --limit 50\n  slk files list --channel '#general' --all\n  slk files list --user @alice --cursor NEXT",
	RunE:    runFilesList,
}
var filesInfoCmd = &cobra.Command{Use: "info", Short: "Get file metadata", Example: "  slk files info --file F123", RunE: runFilesInfo}
var filesDeleteCmd = &cobra.Command{Use: "delete", Short: "Delete a file", Example: "  slk files delete --file F123", RunE: runFilesDelete}
var filesSharePublicCmd = &cobra.Command{Use: "share-public", Short: "Create a public URL for a file", Example: "  slk files share-public --file F123", RunE: runFilesSharePublic}
var filesRevokePublicCmd = &cobra.Command{Use: "revoke-public", Short: "Revoke a file's public URL", Example: "  slk files revoke-public --file F123", RunE: runFilesRevokePublic}

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(filesUploadCmd, filesDownloadCmd, filesListCmd, filesInfoCmd, filesDeleteCmd, filesSharePublicCmd, filesRevokePublicCmd)

	filesUploadCmd.Flags().String("file", "", "Local file path (required)")
	filesUploadCmd.Flags().StringP("channel", "c", "", "Channel name or ID to share into")
	filesUploadCmd.Flags().String("thread", "", "Thread timestamp when sharing into a channel")
	filesUploadCmd.Flags().String("title", "", "Slack file title")
	filesUploadCmd.Flags().String("comment", "", "Initial comment when sharing")
	filesUploadCmd.Flags().String("alt-text", "", "Accessible alt text")
	filesUploadCmd.Flags().String("snippet-type", "", "Slack snippet type for text files")
	_ = filesUploadCmd.MarkFlagRequired("file")

	filesDownloadCmd.Flags().String("file", "", "Slack file ID (required)")
	filesDownloadCmd.Flags().StringP("output", "o", "", "Destination path (required)")
	filesDownloadCmd.Flags().Bool("force", false, "Overwrite an existing destination")
	_ = filesDownloadCmd.MarkFlagRequired("file")
	_ = filesDownloadCmd.MarkFlagRequired("output")

	filesListCmd.Flags().IntP("limit", "l", 100, "Maximum files per page")
	filesListCmd.Flags().String("cursor", "", "Continuation cursor")
	filesListCmd.Flags().Bool("all", false, "Fetch all remaining pages")
	filesListCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack rate limits")
	filesListCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")
	filesListCmd.Flags().StringP("channel", "c", "", "Filter by channel name or ID")
	filesListCmd.Flags().String("user", "", "Filter by canonical user ID, <@ID>, or @username")
	filesListCmd.Flags().String("type", "", "Filter by Slack file type (for example text, images, or canvas)")

	for _, command := range []*cobra.Command{filesInfoCmd, filesDeleteCmd, filesSharePublicCmd, filesRevokePublicCmd} {
		command.Flags().String("file", "", "Slack file ID (required)")
		_ = command.MarkFlagRequired("file")
	}
}

func runFilesUpload(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	path, _ := cmd.Flags().GetString("file")
	channel, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if channel != "" {
		channelID, err = cmdCtx.ResolveChannel(channel)
		if err != nil {
			return err
		}
	}
	thread, _ := cmd.Flags().GetString("thread")
	if thread != "" && channelID == "" {
		return fmt.Errorf("--thread requires --channel")
	}
	title, _ := cmd.Flags().GetString("title")
	comment, _ := cmd.Flags().GetString("comment")
	altText, _ := cmd.Flags().GetString("alt-text")
	snippetType, _ := cmd.Flags().GetString("snippet-type")
	result, err := cmdCtx.Client.UploadLocalFile(cmdCtx.Ctx, path, appslack.UploadFileOptions{Channel: channelID, ThreadTS: thread, Title: title, InitialComment: comment, AltText: altText, SnippetType: snippetType})
	if err != nil {
		return err
	}
	if channel != "" {
		result.Channel = channel
	}
	return output.Print(cmd, result)
}

func runFilesDownload(cmd *cobra.Command, _ []string) (err error) {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileID, _ := cmd.Flags().GetString("file")
	destination, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("--output is required")
	}
	destination = filepath.Clean(destination)
	if !force {
		if _, statErr := os.Stat(destination); statErr == nil {
			return fmt.Errorf("download destination already exists (use --force to replace it): %s", destination)
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect download destination: %w", statErr)
		}
	}
	directory := filepath.Dir(destination)
	writer, err := os.CreateTemp(directory, ".slk-download-*")
	if err != nil {
		return fmt.Errorf("create temporary download: %w", err)
	}
	temporary := writer.Name()
	if err := writer.Chmod(0o600); err != nil {
		_ = writer.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("secure temporary download: %w", err)
	}
	complete := false
	defer func() {
		_ = writer.Close()
		if !complete {
			_ = os.Remove(temporary)
		}
	}()
	service := fileops.NewService(cmdCtx.Client)
	file, err := service.Download(cmdCtx.Ctx, fileID, writer)
	if err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close download: %w", err)
	}
	if force {
		if err := os.Rename(temporary, destination); err != nil {
			return fmt.Errorf("replace download destination: %w", err)
		}
	} else {
		// A hard link makes the no-overwrite guarantee atomic even if another
		// process creates the destination after the preflight check.
		if err := os.Link(temporary, destination); err != nil {
			return fmt.Errorf("install download destination: %w", err)
		}
		if err := os.Remove(temporary); err != nil {
			_ = os.Remove(destination)
			return fmt.Errorf("finalize download destination: %w", err)
		}
	}
	complete = true
	return output.Print(cmd, &downloadResult{OK: true, FileID: file.ID, Path: destination, Size: file.Size})
}

type downloadResult struct {
	OK     bool   `json:"ok"`
	FileID string `json:"file_id"`
	Path   string `json:"path"`
	Size   int    `json:"size"`
}

func (r *downloadResult) Lines() []string {
	return []string{"File downloaded", "File ID: " + r.FileID, "Path: " + r.Path, fmt.Sprintf("Size: %d bytes", r.Size)}
}

func runFilesList(cmd *cobra.Command, _ []string) error {
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
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if maxRetries < 0 || pageDelay < 0 {
		return fmt.Errorf("--max-retries and --page-delay cannot be negative")
	}
	channel, _ := cmd.Flags().GetString("channel")
	if channel != "" {
		channel, err = cmdCtx.ResolveChannel(channel)
		if err != nil {
			return err
		}
	}
	user, _ := cmd.Flags().GetString("user")
	if user != "" {
		user, err = resolveUserID(cmdCtx.Ctx, cmdCtx.Client, user)
		if err != nil {
			return err
		}
	}
	types, _ := cmd.Flags().GetString("type")
	service := fileops.NewService(cmdCtx.Client)
	combined := &fileops.ListResult{OK: true, Files: []slackapi.File{}}
	seen := map[string]bool{}
	for {
		page, err := appslack.RetryRateLimited(cmdCtx.Ctx, maxRetries, func() (*fileops.ListResult, error) {
			return service.List(cmdCtx.Ctx, fileops.ListParams{Limit: limit, Cursor: cursor, User: user, Channel: channel, TeamID: cmdCtx.TeamID, Types: types})
		})
		if err != nil {
			return err
		}
		combined.Files = append(combined.Files, page.Files...)
		combined.NextCursor = page.NextCursor
		if !all || page.NextCursor == "" {
			break
		}
		if seen[page.NextCursor] {
			return fmt.Errorf("Slack returned repeated file cursor %q", page.NextCursor)
		}
		seen[page.NextCursor] = true
		if err := appslack.WaitContext(cmdCtx.Ctx, pageDelay); err != nil {
			return err
		}
		cursor = page.NextCursor
	}
	return output.Print(cmd, combined)
}

func runFilesInfo(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileID, _ := cmd.Flags().GetString("file")
	result, err := fileops.NewService(cmdCtx.Client).Info(cmdCtx.Ctx, fileID)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}
func runFilesDelete(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileID, _ := cmd.Flags().GetString("file")
	result, err := fileops.NewService(cmdCtx.Client).Delete(cmdCtx.Ctx, fileID)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}
func runFilesSharePublic(cmd *cobra.Command, _ []string) error  { return runFilesSetPublic(cmd, true) }
func runFilesRevokePublic(cmd *cobra.Command, _ []string) error { return runFilesSetPublic(cmd, false) }
func runFilesSetPublic(cmd *cobra.Command, public bool) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileID, _ := cmd.Flags().GetString("file")
	result, err := fileops.NewService(cmdCtx.Client).SetPublic(cmdCtx.Ctx, fileID, public)
	if err != nil {
		return err
	}
	return output.Print(cmd, result)
}
