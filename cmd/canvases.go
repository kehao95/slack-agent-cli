package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/content"
	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var canvasesCmd = &cobra.Command{Use: "canvases", Short: "Manage Slack canvases"}
var canvasesListCmd = &cobra.Command{Use: "list", Short: "List canvases", RunE: runCanvasesList}
var canvasesCreateCmd = &cobra.Command{Use: "create", Short: "Create a standalone canvas", RunE: runCanvasesCreate}
var canvasesChannelCreateCmd = &cobra.Command{Use: "channel-create", Short: "Create a channel canvas", RunE: runCanvasesChannelCreate}
var canvasesReadCmd = &cobra.Command{Use: "read", Short: "Read canvas content", RunE: runCanvasesRead}
var canvasesExportCmd = &cobra.Command{Use: "export", Short: "Export canvas content to a file", RunE: runCanvasesExport}
var canvasesEditCmd = &cobra.Command{Use: "edit", Short: "Apply edits to a canvas", RunE: runCanvasesEdit}
var canvasesDeleteCmd = &cobra.Command{Use: "delete", Short: "Delete a canvas", RunE: runCanvasesDelete}
var canvasesSectionsCmd = &cobra.Command{Use: "sections", Short: "Find canvas sections", RunE: runCanvasesSections}
var canvasesShareCmd = &cobra.Command{Use: "share", Short: "Share a canvas", RunE: runCanvasesShare}
var canvasesRevokeCmd = &cobra.Command{Use: "revoke", Short: "Revoke canvas access", RunE: runCanvasesRevoke}

func init() {
	rootCmd.AddCommand(canvasesCmd)
	canvasesCmd.AddCommand(canvasesListCmd, canvasesCreateCmd, canvasesChannelCreateCmd, canvasesReadCmd, canvasesExportCmd, canvasesEditCmd, canvasesDeleteCmd, canvasesSectionsCmd, canvasesShareCmd, canvasesRevokeCmd)

	canvasesListCmd.Flags().Int("limit", 100, "Maximum canvases per page")
	canvasesListCmd.Flags().Int("page", 1, "Page number")
	canvasesListCmd.Flags().Bool("all", false, "Fetch every page")

	canvasesCreateCmd.Flags().String("title", "", "Canvas title")
	canvasesCreateCmd.Flags().StringP("channel", "c", "", "Channel name or ID to tab the canvas in")
	canvasesCreateCmd.Flags().String("content", "", "document_content JSON, @file, or - for stdin")
	canvasesChannelCreateCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
	canvasesChannelCreateCmd.Flags().String("title", "", "Canvas title")
	canvasesChannelCreateCmd.Flags().String("content", "", "document_content JSON, @file, or - for stdin")

	for _, command := range []*cobra.Command{canvasesReadCmd, canvasesExportCmd} {
		command.Flags().String("canvas", "", "Canvas ID (required)")
		command.Flags().String("format", "markdown", "Output format: markdown or html")
	}
	canvasesReadCmd.Flags().StringP("output", "o", "", "Optional output path")
	canvasesExportCmd.Flags().StringP("output", "o", "", "Destination path (required)")
	canvasesExportCmd.Flags().Bool("force", false, "Overwrite an existing destination")

	canvasesEditCmd.Flags().String("canvas", "", "Canvas ID (required)")
	canvasesEditCmd.Flags().String("changes", "", "Changes JSON array, @file, or - for stdin (required)")
	canvasesDeleteCmd.Flags().String("canvas", "", "Canvas ID (required)")
	canvasesSectionsCmd.Flags().String("canvas", "", "Canvas ID (required)")
	canvasesSectionsCmd.Flags().String("criteria", "", "Section criteria JSON object, @file, or - for stdin (required)")

	canvasesShareCmd.Flags().String("canvas", "", "Canvas ID (required)")
	canvasesShareCmd.Flags().String("access-level", "", "Access level: read, write, or owner (required)")
	canvasesShareCmd.Flags().String("channels", "", "Comma-separated channel names or IDs")
	canvasesShareCmd.Flags().String("users", "", "Comma-separated user IDs, <@ID> mentions, or @usernames")
	canvasesRevokeCmd.Flags().String("canvas", "", "Canvas ID (required)")
	canvasesRevokeCmd.Flags().String("channels", "", "Comma-separated channel names or IDs")
	canvasesRevokeCmd.Flags().String("users", "", "Comma-separated user IDs, <@ID> mentions, or @usernames")
}

type canvasReadResult struct {
	OK        bool   `json:"ok"`
	CanvasID  string `json:"canvas_id"`
	Format    string `json:"format"`
	Content   string `json:"content"`
	Converted bool   `json:"converted"`
	Lossy     bool   `json:"lossy"`
	Output    string `json:"output,omitempty"`
}

func (r canvasReadResult) Lines() []string {
	if r.Output != "" {
		return []string{"Canvas exported", "Canvas ID: " + r.CanvasID, "Format: " + r.Format, "Path: " + r.Output}
	}
	return []string{r.Content}
}

func parseArtifactObject(value, flag string) (map[string]interface{}, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("--%s is required", flag)
	}
	resolved, err := readJSONArgument(value)
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(resolved), &object); err != nil {
		return nil, fmt.Errorf("invalid --%s JSON object: %w", flag, err)
	}
	if object == nil {
		return nil, fmt.Errorf("--%s must be a JSON object", flag)
	}
	return object, nil
}

func parseArtifactArray(value, flag string) ([]interface{}, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("--%s is required", flag)
	}
	resolved, err := readJSONArgument(value)
	if err != nil {
		return nil, err
	}
	var array []interface{}
	if err := json.Unmarshal([]byte(resolved), &array); err != nil {
		return nil, fmt.Errorf("invalid --%s JSON array: %w", flag, err)
	}
	if len(array) == 0 {
		return nil, fmt.Errorf("--%s cannot be empty", flag)
	}
	return array, nil
}

func parseCanvasContent(value string) (map[string]interface{}, error) {
	object, err := parseArtifactObject(value, "content")
	if err != nil {
		return nil, err
	}
	typeName, _ := object["type"].(string)
	if strings.TrimSpace(typeName) != "markdown" {
		return nil, fmt.Errorf("canvas content type must be markdown")
	}
	if _, ok := object["markdown"].(string); !ok {
		return nil, fmt.Errorf("canvas markdown content requires a markdown string")
	}
	return object, nil
}

func runCanvasesCreate(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channel := artifactFlagString(cmd, "channel")
	channelID := ""
	if channel != "" {
		channelID, err = ctx.ResolveChannel(channel)
		if err != nil {
			return err
		}
	}
	var document map[string]interface{}
	if value := artifactFlagString(cmd, "content"); value != "" {
		document, err = parseCanvasContent(value)
		if err != nil {
			return err
		}
	}
	result, err := ctx.Client.CreateCanvas(ctx.Ctx, appslack.CanvasCreateParams{Title: artifactFlagString(cmd, "title"), ChannelID: channelID, DocumentContent: document})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesChannelCreate(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	channel := artifactFlagString(cmd, "channel")
	if channel == "" {
		return fmt.Errorf("--channel is required")
	}
	channelID, err := ctx.ResolveChannel(channel)
	if err != nil {
		return err
	}
	var document map[string]interface{}
	if value := artifactFlagString(cmd, "content"); value != "" {
		document, err = parseCanvasContent(value)
		if err != nil {
			return err
		}
	}
	result, err := ctx.Client.CreateChannelCanvas(ctx.Ctx, appslack.CanvasCreateChannelParams{ChannelID: channelID, Title: artifactFlagString(cmd, "title"), DocumentContent: document})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesList(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	limit, _ := cmd.Flags().GetInt("limit")
	page, _ := cmd.Flags().GetInt("page")
	all, _ := cmd.Flags().GetBool("all")
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}
	if page < 1 {
		return fmt.Errorf("--page must be at least 1")
	}
	var pages []json.RawMessage
	for {
		result, err := ctx.Client.ListCanvases(ctx.Ctx, limit, strconv.Itoa(page))
		if err != nil {
			return err
		}
		pages = append(pages, json.RawMessage(result))
		if !all {
			break
		}
		var envelope struct {
			Paging struct {
				Page  int `json:"page"`
				Pages int `json:"pages"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(result, &envelope); err != nil || envelope.Paging.Pages == 0 || page >= envelope.Paging.Pages {
			break
		}
		page++
	}
	if len(pages) == 1 {
		return printArtifactResponse(cmd, appslack.ArtifactResponse(pages[0]))
	}
	return printArtifactResponse(cmd, appslack.ArtifactResponse(mustJSON(map[string]interface{}{"ok": true, "page_count": len(pages), "pages": pages})))
}

func runCanvasesRead(cmd *cobra.Command, _ []string) error {
	return runCanvasContent(cmd, false)
}

func runCanvasesExport(cmd *cobra.Command, _ []string) error {
	return runCanvasContent(cmd, true)
}

func runCanvasContent(cmd *cobra.Command, export bool) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	format := strings.ToLower(artifactFlagString(cmd, "format"))
	if format != "markdown" && format != "html" {
		return fmt.Errorf("--format must be markdown or html")
	}
	outputPath := artifactFlagString(cmd, "output")
	if export && outputPath == "" {
		return fmt.Errorf("--output is required")
	}
	raw := &limitedCanvasBuffer{limit: 16 << 20}
	_, err = ctx.Client.DownloadCanvas(ctx.Ctx, canvasID, raw)
	if err != nil {
		return err
	}
	data := raw.buf.String()
	converted, lossy := false, false
	if format == "markdown" {
		data, err = content.HTMLToMarkdown(data)
		if err != nil {
			return err
		}
		converted, lossy = true, true
	}
	if outputPath != "" {
		if err := writeArtifactFile(outputPath, []byte(data), flagBool(cmd, "force")); err != nil {
			return err
		}
	}
	result := canvasReadResult{OK: true, CanvasID: canvasID, Format: format, Converted: converted, Lossy: lossy}
	if outputPath != "" {
		result.Output = filepath.Clean(outputPath)
	} else {
		result.Content = data
	}
	return output.Print(cmd, result)
}

type limitedCanvasBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *limitedCanvasBuffer) Write(data []byte) (int, error) {
	if b.buf.Len()+len(data) > b.limit {
		return 0, fmt.Errorf("canvas download exceeds %d MiB limit", b.limit/(1<<20))
	}
	return b.buf.Write(data)
}

func runCanvasesEdit(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	changes, err := parseArtifactArray(artifactFlagString(cmd, "changes"), "changes")
	if err != nil {
		return err
	}
	result, err := ctx.Client.EditCanvas(ctx.Ctx, appslack.CanvasEditParams{CanvasID: canvasID, Changes: changes})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesDelete(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	result, err := ctx.Client.DeleteCanvas(ctx.Ctx, canvasID)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesSections(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	criteria, err := parseArtifactObject(artifactFlagString(cmd, "criteria"), "criteria")
	if err != nil {
		return err
	}
	result, err := ctx.Client.LookupCanvasSections(ctx.Ctx, canvasID, criteria)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesShare(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	level := artifactFlagString(cmd, "access-level")
	if level != "read" && level != "write" && level != "owner" {
		return fmt.Errorf("--access-level must be read, write, or owner")
	}
	channels, users, err := resolveArtifactTargets(cmd, ctx)
	if err != nil {
		return err
	}
	if len(channels) == 0 && len(users) == 0 {
		return fmt.Errorf("one of --channels or --users is required")
	}
	if len(channels) > 0 && len(users) > 0 {
		return fmt.Errorf("--channels and --users cannot be combined")
	}
	if level == "owner" && len(channels) > 0 {
		return fmt.Errorf("owner access requires --users")
	}
	result, err := ctx.Client.SetCanvasAccess(ctx.Ctx, appslack.CanvasAccessParams{CanvasID: canvasID, AccessLevel: level, ChannelIDs: channels, UserIDs: users})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runCanvasesRevoke(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	canvasID := artifactFlagString(cmd, "canvas")
	if canvasID == "" {
		return fmt.Errorf("--canvas is required")
	}
	channels, users, err := resolveArtifactTargets(cmd, ctx)
	if err != nil {
		return err
	}
	if len(channels) == 0 && len(users) == 0 {
		return fmt.Errorf("one of --channels or --users is required")
	}
	if len(channels) > 0 && len(users) > 0 {
		return fmt.Errorf("--channels and --users cannot be combined")
	}
	result, err := ctx.Client.DeleteCanvasAccess(ctx.Ctx, canvasID, channels, users)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func resolveArtifactTargets(cmd *cobra.Command, ctx *CommandContext) ([]string, []string, error) {
	channelRefs, err := commaArtifactValues(artifactFlagString(cmd, "channels"), "channels")
	if err != nil {
		return nil, nil, err
	}
	userRefs, err := commaArtifactValues(artifactFlagString(cmd, "users"), "users")
	if err != nil {
		return nil, nil, err
	}
	channels := make([]string, 0, len(channelRefs))
	for _, ref := range channelRefs {
		id, err := ctx.ResolveChannel(ref)
		if err != nil {
			return nil, nil, err
		}
		channels = append(channels, id)
	}
	users := make([]string, 0, len(userRefs))
	for _, ref := range userRefs {
		id, err := ctx.ResolveUser(ref)
		if err != nil {
			return nil, nil, err
		}
		users = append(users, id)
	}
	return channels, users, nil
}

func commaArtifactValues(value, flag string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("--%s contains an empty value", flag)
		}
		result = append(result, part)
	}
	return result, nil
}

func flagBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func writeArtifactFile(path string, data []byte, force bool) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return fmt.Errorf("output path is required")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output destination already exists (use --force to replace it): %s", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect output destination: %w", err)
		}
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".slk-artifact-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = file.Close(); _ = os.Remove(temporary) }()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure output: %w", err)
	}
	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if force {
		if err := os.Rename(temporary, path); err != nil {
			return fmt.Errorf("replace output: %w", err)
		}
		return nil
	}
	if err := os.Link(temporary, path); err != nil {
		return fmt.Errorf("install output: %w", err)
	}
	return nil
}

func mustJSON(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}
