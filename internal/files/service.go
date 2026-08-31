// Package files provides resource-oriented Slack file operations.
package files

import (
	"context"
	"fmt"
	"io"
	"strings"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type Client interface {
	UploadLocalFile(context.Context, string, appslack.UploadFileOptions) (*appslack.UploadFileResult, error)
	ListFiles(context.Context, slackapi.ListFilesParameters) ([]slackapi.File, string, error)
	GetFileInfo(context.Context, string) (*slackapi.File, error)
	DownloadFile(context.Context, string, io.Writer) error
	DeleteFile(context.Context, string) error
	ShareFilePublicURL(context.Context, string) (*slackapi.File, error)
	RevokeFilePublicURL(context.Context, string) (*slackapi.File, error)
}

type Service struct{ client Client }

func NewService(client Client) *Service { return &Service{client: client} }

type ListParams struct {
	Limit   int
	Cursor  string
	User    string
	Channel string
	TeamID  string
}

type ListResult struct {
	OK         bool            `json:"ok"`
	Files      []slackapi.File `json:"files"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}
	items, cursor, err := s.client.ListFiles(ctx, slackapi.ListFilesParameters{
		Limit: params.Limit, Cursor: params.Cursor, User: params.User, Channel: params.Channel, TeamID: params.TeamID,
	})
	if err != nil {
		return nil, err
	}
	return &ListResult{OK: true, Files: items, NextCursor: cursor}, nil
}

type InfoResult struct {
	OK   bool          `json:"ok"`
	File slackapi.File `json:"file"`
}

func (s *Service) Info(ctx context.Context, fileID string) (*InfoResult, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("file ID is required")
	}
	file, err := s.client.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return &InfoResult{OK: true, File: *file}, nil
}

func (s *Service) Download(ctx context.Context, fileID string, writer io.Writer) (*slackapi.File, error) {
	file, err := s.client.GetFileInfo(ctx, strings.TrimSpace(fileID))
	if err != nil {
		return nil, err
	}
	url := file.URLPrivateDownload
	if url == "" {
		url = file.URLPrivate
	}
	if url == "" {
		return nil, fmt.Errorf("file %s has no private download URL", fileID)
	}
	if err := s.client.DownloadFile(ctx, url, writer); err != nil {
		return nil, err
	}
	return file, nil
}

type MutationResult struct {
	OK              bool   `json:"ok"`
	Action          string `json:"action"`
	FileID          string `json:"file_id"`
	PermalinkPublic string `json:"permalink_public,omitempty"`
}

func (s *Service) Delete(ctx context.Context, fileID string) (*MutationResult, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("file ID is required")
	}
	if err := s.client.DeleteFile(ctx, fileID); err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "delete", FileID: fileID}, nil
}

func (s *Service) SetPublic(ctx context.Context, fileID string, public bool) (*MutationResult, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("file ID is required")
	}
	var file *slackapi.File
	var err error
	if public {
		file, err = s.client.ShareFilePublicURL(ctx, fileID)
	} else {
		file, err = s.client.RevokeFilePublicURL(ctx, fileID)
	}
	if err != nil {
		return nil, err
	}
	action := "revoke-public"
	if public {
		action = "share-public"
	}
	return &MutationResult{OK: true, Action: action, FileID: fileID, PermalinkPublic: file.PermalinkPublic}, nil
}

func (r *ListResult) Lines() []string {
	if len(r.Files) == 0 {
		return []string{"No files found."}
	}
	lines := []string{fmt.Sprintf("Files (%d)", len(r.Files))}
	for _, file := range r.Files {
		lines = append(lines, fmt.Sprintf("%s  %s  %d bytes", file.ID, firstNonEmpty(file.Title, file.Name), file.Size))
	}
	if r.NextCursor != "" {
		lines = append(lines, "", "Next cursor: "+r.NextCursor)
	}
	return lines
}

func (r *InfoResult) Lines() []string {
	return []string{fmt.Sprintf("File: %s", firstNonEmpty(r.File.Title, r.File.Name)), "ID: " + r.File.ID, fmt.Sprintf("Type: %s", firstNonEmpty(r.File.PrettyType, r.File.Mimetype)), fmt.Sprintf("Size: %d bytes", r.File.Size), "Permalink: " + r.File.Permalink}
}

func (r *MutationResult) Lines() []string {
	return []string{fmt.Sprintf("File %s: %s", r.Action, r.FileID)}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "(untitled)"
}
