package slack

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// UploadFileOptions controls a generic Slack file upload.
type UploadFileOptions struct {
	Channel        string
	ThreadTS       string
	Title          string
	InitialComment string
	AltText        string
	SnippetType    string
}

// UploadFileResult describes a completed external file upload.
type UploadFileResult struct {
	OK       bool   `json:"ok"`
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	Title    string `json:"title,omitempty"`
	Channel  string `json:"channel,omitempty"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

func (r *UploadFileResult) Lines() []string {
	lines := []string{"File uploaded successfully", fmt.Sprintf("File ID: %s", r.FileID), fmt.Sprintf("Filename: %s", r.Filename)}
	if r.Channel != "" {
		lines = append(lines, fmt.Sprintf("Channel: %s", r.Channel))
	}
	if r.ThreadTS != "" {
		lines = append(lines, fmt.Sprintf("Thread: %s", r.ThreadTS))
	}
	return lines
}

// UploadLocalFile uploads a non-empty local file and optionally shares it.
func (c *APIClient) UploadLocalFile(ctx context.Context, path string, opts UploadFileOptions) (*UploadFileResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("file path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("file path is a directory: %s", path)
	}
	if info.Size() <= 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}
	if info.Size() > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("file is too large: %s", path)
	}

	filename := filepath.Base(path)
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = filename
	}
	summary, err := c.sdk.UploadFileContext(ctx, slackapi.UploadFileParameters{
		Reader:          file,
		FileSize:        int(info.Size()),
		Filename:        filename,
		Title:           title,
		Channel:         strings.TrimSpace(opts.Channel),
		ThreadTimestamp: strings.TrimSpace(opts.ThreadTS),
		InitialComment:  opts.InitialComment,
		AltTxt:          opts.AltText,
		SnippetType:     opts.SnippetType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}
	if summary == nil || summary.ID == "" {
		return nil, fmt.Errorf("upload file: Slack returned no file ID")
	}
	return &UploadFileResult{OK: true, FileID: summary.ID, Filename: filename, Title: title, Channel: opts.Channel, ThreadTS: opts.ThreadTS}, nil
}

// ListFiles fetches one cursor-based page of files.
func (c *APIClient) ListFiles(ctx context.Context, params slackapi.ListFilesParameters) ([]slackapi.File, string, error) {
	files, next, err := c.sdk.ListFilesContext(ctx, params)
	if err != nil {
		return nil, "", fmt.Errorf("list files: %w", err)
	}
	if next == nil {
		return files, "", nil
	}
	return files, next.Cursor, nil
}

// GetFileInfo fetches metadata for a Slack file.
func (c *APIClient) GetFileInfo(ctx context.Context, fileID string) (*slackapi.File, error) {
	file, _, _, err := c.sdk.GetFileInfoContext(ctx, strings.TrimSpace(fileID), 0, 0)
	if err != nil {
		return nil, fmt.Errorf("get file info: %w", err)
	}
	return file, nil
}

// DownloadFile streams a private Slack file URL to writer.
func (c *APIClient) DownloadFile(ctx context.Context, downloadURL string, writer io.Writer) error {
	if strings.TrimSpace(downloadURL) == "" {
		return fmt.Errorf("download URL is required")
	}
	if writer == nil {
		return fmt.Errorf("download writer is required")
	}
	if err := c.sdk.GetFileContext(withDownloadPermission(ctx), downloadURL, writer); err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	return nil
}

func (c *APIClient) DeleteFile(ctx context.Context, fileID string) error {
	if err := c.sdk.DeleteFileContext(ctx, strings.TrimSpace(fileID)); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (c *APIClient) ShareFilePublicURL(ctx context.Context, fileID string) (*slackapi.File, error) {
	file, _, _, err := c.sdk.ShareFilePublicURLContext(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return nil, fmt.Errorf("share file public URL: %w", err)
	}
	return file, nil
}

func (c *APIClient) RevokeFilePublicURL(ctx context.Context, fileID string) (*slackapi.File, error) {
	file, err := c.sdk.RevokeFilePublicURLContext(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return nil, fmt.Errorf("revoke file public URL: %w", err)
	}
	return file, nil
}

// UploadImageOptions controls how an image is shared into Slack.
type UploadImageOptions struct {
	ThreadTS       string
	InitialComment string
	AltText        string
	Blocks         []slackapi.Block
}

// UploadImageResult represents an image uploaded and shared into Slack.
// Slack's external upload API returns the file identity rather than a message
// timestamp, so FileID is the stable reference for the uploaded image.
type UploadImageResult struct {
	OK       bool   `json:"ok"`
	Channel  string `json:"channel"`
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

// Lines implements the output.Printable interface for human-readable output.
func (r *UploadImageResult) Lines() []string {
	lines := []string{
		"Image sent successfully",
		fmt.Sprintf("Channel: %s", r.Channel),
		fmt.Sprintf("File ID: %s", r.FileID),
		fmt.Sprintf("Filename: %s", r.Filename),
	}
	if r.ThreadTS != "" {
		lines = append(lines, fmt.Sprintf("Thread: %s", r.ThreadTS))
	}
	return lines
}

// UploadImage uploads a local image using Slack's current external upload
// sequence and shares it to a channel, optionally as a thread reply.
func (c *APIClient) UploadImage(ctx context.Context, channel, path string, opts UploadImageOptions) (*UploadImageResult, error) {
	if channel == "" {
		return nil, ErrChannelRequired
	}
	if path == "" {
		return nil, fmt.Errorf("image path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("image path is a directory: %s", path)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("image file is empty: %s", path)
	}
	if info.Size() > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("image file is too large: %s", path)
	}

	filename := filepath.Base(path)
	summary, err := c.sdk.UploadFileContext(ctx, slackapi.UploadFileParameters{
		Reader:          file,
		FileSize:        int(info.Size()),
		Filename:        filename,
		Title:           filename,
		AltTxt:          opts.AltText,
		Channel:         channel,
		ThreadTimestamp: opts.ThreadTS,
		InitialComment:  opts.InitialComment,
		Blocks:          slackapi.Blocks{BlockSet: opts.Blocks},
	})
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}
	if summary == nil || summary.ID == "" {
		return nil, fmt.Errorf("upload image: Slack returned no file ID")
	}

	title := summary.Title
	if title == "" {
		title = filename
	}
	return &UploadImageResult{
		OK:       true,
		Channel:  channel,
		FileID:   summary.ID,
		Filename: filename,
		Title:    title,
		ThreadTS: opts.ThreadTS,
	}, nil
}
