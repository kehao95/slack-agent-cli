package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	fileops "github.com/kehao95/slack-agent-cli/internal/files"
	"github.com/kehao95/slack-agent-cli/internal/output"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

var filesReadCmd = &cobra.Command{Use: "read", Short: "Read a text Slack file", RunE: runFilesRead}
var filesExportCmd = &cobra.Command{Use: "export", Short: "Export Slack file content", RunE: runFilesExport}

func init() {
	filesCmd.AddCommand(filesReadCmd, filesExportCmd)
	for _, command := range []*cobra.Command{filesReadCmd, filesExportCmd} {
		command.Flags().String("file", "", "Slack file ID or Slack file URL (required)")
		_ = command.MarkFlagRequired("file")
		command.Flags().Bool("raw", false, "Write exact content to stdout instead of JSON")
	}
	filesExportCmd.Flags().StringP("output", "o", "", "Destination path; stdout when omitted")
	filesExportCmd.Flags().Bool("force", false, "Overwrite an existing destination")
}

type fileReadResult struct {
	OK       bool   `json:"ok"`
	FileID   string `json:"file_id"`
	Name     string `json:"name,omitempty"`
	Mimetype string `json:"mimetype,omitempty"`
	Content  string `json:"content"`
	Bytes    int    `json:"bytes"`
}

func (r *fileReadResult) Lines() []string {
	return []string{fmt.Sprintf("File: %s", r.Name), fmt.Sprintf("ID: %s", r.FileID), fmt.Sprintf("Bytes: %d", r.Bytes), r.Content}
}

func runFilesRead(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileRef, _ := cmd.Flags().GetString("file")
	raw, _ := cmd.Flags().GetBool("raw")
	service := fileops.NewService(cmdCtx.Client)
	if raw {
		_, err := service.Download(cmdCtx.Ctx, fileRef, cmd.OutOrStdout())
		return err
	}
	file, content, err := service.ReadText(cmdCtx.Ctx, fileRef)
	if err != nil {
		return err
	}
	return output.Print(cmd, &fileReadResult{OK: true, FileID: file.ID, Name: firstFileName(file.Name, file.Title), Mimetype: file.Mimetype, Content: content, Bytes: len([]byte(content))})
}

func runFilesExport(cmd *cobra.Command, _ []string) error {
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	fileRef, _ := cmd.Flags().GetString("file")
	destination, _ := cmd.Flags().GetString("output")
	raw, _ := cmd.Flags().GetBool("raw")
	service := fileops.NewService(cmdCtx.Client)
	if strings.TrimSpace(destination) != "" {
		force, _ := cmd.Flags().GetBool("force")
		file, size, err := installRemoteFile(cmdCtx.Ctx, service, fileRef, destination, force)
		if err != nil {
			return err
		}
		return output.Print(cmd, &fileExportResult{OK: true, FileID: file.ID, Path: filepath.Clean(destination), Bytes: size})
	}
	if raw {
		_, err := service.Download(cmdCtx.Ctx, fileRef, cmd.OutOrStdout())
		return err
	}
	file, content, err := service.ReadBytes(cmdCtx.Ctx, fileRef)
	if err != nil {
		return err
	}
	if !utf8.Valid(content) {
		return fmt.Errorf("file %s is not UTF-8; use --raw or --output for binary content", file.ID)
	}
	return output.Print(cmd, &fileReadResult{OK: true, FileID: file.ID, Name: firstFileName(file.Name, file.Title), Mimetype: file.Mimetype, Content: string(content), Bytes: len(content)})
}

func installRemoteFile(ctx context.Context, service *fileops.Service, reference, destination string, force bool) (*slackapi.File, int, error) {
	destination = filepath.Clean(destination)
	if !force {
		if _, err := os.Stat(destination); err == nil {
			return nil, 0, fmt.Errorf("destination already exists (use --force to replace it): %s", destination)
		} else if !os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("inspect destination: %w", err)
		}
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(destination), ".slk-export-*")
	if err != nil {
		return nil, 0, fmt.Errorf("create temporary export: %w", err)
	}
	temporary := temporaryFile.Name()
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(temporary)
		}
	}()
	if err := temporaryFile.Chmod(0o600); err != nil {
		_ = temporaryFile.Close()
		return nil, 0, fmt.Errorf("secure temporary export: %w", err)
	}
	file, err := service.Download(ctx, reference, temporaryFile)
	if err != nil {
		_ = temporaryFile.Close()
		return nil, 0, err
	}
	if err := temporaryFile.Close(); err != nil {
		return nil, 0, fmt.Errorf("close temporary export: %w", err)
	}
	if force {
		if err := os.Rename(temporary, destination); err != nil {
			return nil, 0, fmt.Errorf("replace export destination: %w", err)
		}
	} else if err := os.Link(temporary, destination); err != nil {
		return nil, 0, fmt.Errorf("install export destination: %w", err)
	} else {
		_ = os.Remove(temporary)
	}
	complete = true
	info, err := os.Stat(destination)
	if err != nil {
		return nil, 0, fmt.Errorf("stat export destination: %w", err)
	}
	return file, int(info.Size()), nil
}

type fileExportResult struct {
	OK     bool   `json:"ok"`
	FileID string `json:"file_id"`
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
}

func (r *fileExportResult) Lines() []string {
	return []string{fmt.Sprintf("File exported: %s", r.Path), fmt.Sprintf("ID: %s", r.FileID), fmt.Sprintf("Bytes: %d", r.Bytes)}
}

func firstFileName(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "(untitled)"
}
