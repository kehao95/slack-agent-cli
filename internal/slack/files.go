package slack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	slackapi "github.com/slack-go/slack"
)

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
