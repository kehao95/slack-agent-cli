// Package files provides resource-oriented Slack file operations.
package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

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
	Types   string
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
		Limit: params.Limit, Cursor: params.Cursor, User: params.User, Channel: params.Channel, TeamID: params.TeamID, Types: params.Types,
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
	fileID, err := ResolveFileID(fileID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("file ID is required")
	}
	file, err := s.client.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("Slack returned no file metadata for %s", fileID)
	}
	return &InfoResult{OK: true, File: *file}, nil
}

func (s *Service) Download(ctx context.Context, fileID string, writer io.Writer) (*slackapi.File, error) {
	resolvedID, err := ResolveFileID(fileID)
	if err != nil {
		return nil, err
	}
	file, err := s.client.GetFileInfo(ctx, resolvedID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("Slack returned no file metadata for %s", resolvedID)
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

// ReadBytes retrieves file metadata and its private content through the authenticated client.
// Callers such as Canvas own any format conversion; this helper intentionally returns bytes.
func (s *Service) ReadBytes(ctx context.Context, fileID string) (*slackapi.File, []byte, error) {
	const maxReadBytes = 16 << 20
	content := &boundedBuffer{limit: maxReadBytes + 1}
	file, err := s.Download(ctx, fileID, content)
	if err != nil {
		return nil, nil, err
	}
	if content.Len() > maxReadBytes {
		return nil, nil, fmt.Errorf("file %s exceeds the 16 MiB read limit; use files download or export --output", file.ID)
	}
	return file, content.Bytes(), nil
}

type boundedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > b.limit {
		remaining := b.limit - b.Len()
		if remaining > 0 {
			_, _ = b.buf.Write(data[:remaining])
		}
		return remaining, fmt.Errorf("content exceeds the 16 MiB read limit; use files download or export --output")
	}
	return b.buf.Write(data)
}

func (b *boundedBuffer) Len() int      { return b.buf.Len() }
func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }

// ReadText retrieves a UTF-8 Slack file and preserves its content exactly.
func (s *Service) ReadText(ctx context.Context, fileID string) (*slackapi.File, string, error) {
	file, data, err := s.ReadBytes(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	if !utf8.Valid(data) {
		return nil, "", fmt.Errorf("file %s is not valid UTF-8 text; use download for binary content", file.ID)
	}
	return file, string(data), nil
}

var fileIDPattern = regexp.MustCompile(`^F[A-Z0-9]+$`)

// ResolveFileID accepts a canonical Slack file ID or a Slack file URL.
func ResolveFileID(reference string) (string, error) {
	trimmed := strings.TrimSpace(reference)
	if fileIDPattern.MatchString(strings.ToUpper(trimmed)) && strings.ToUpper(trimmed) == trimmed {
		return trimmed, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" || u.User != nil || u.Port() != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("file must be a Slack file ID or Slack URL")
	}
	host := strings.ToLower(u.Hostname())
	if host != "slack.com" && !strings.HasSuffix(host, ".slack.com") {
		return "", fmt.Errorf("file URL must point to Slack")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var candidates []string
	switch {
	case len(parts) >= 3 && parts[0] == "files":
		// Slack permalink: /files/<user-id>/<file-id>/<title>
		candidates = append(candidates, parts[2])
	case len(parts) >= 2 && parts[0] == "files-pri":
		// Private file URL: /files-pri/<team-id>-<file-id>/<name>
		candidates = append(candidates, strings.Split(parts[1], "-")...)
	case len(parts) >= 4 && parts[0] == "client" && parts[1] == "files":
		// Slack client URL: /client/files/<team-id>/<file-id>/...
		candidates = append(candidates, parts[3])
	}
	for _, candidate := range candidates {
		candidate = strings.ToUpper(strings.TrimSpace(candidate))
		if fileIDPattern.MatchString(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Slack file URL does not contain a file ID")
}

type MutationResult struct {
	OK              bool   `json:"ok"`
	Action          string `json:"action"`
	FileID          string `json:"file_id"`
	PermalinkPublic string `json:"permalink_public,omitempty"`
}

func (s *Service) Delete(ctx context.Context, fileID string) (*MutationResult, error) {
	resolvedID, err := ResolveFileID(fileID)
	if err != nil {
		return nil, err
	}
	fileID = resolvedID
	if err := s.client.DeleteFile(ctx, fileID); err != nil {
		return nil, err
	}
	return &MutationResult{OK: true, Action: "delete", FileID: fileID}, nil
}

func (s *Service) SetPublic(ctx context.Context, fileID string, public bool) (*MutationResult, error) {
	resolvedID, err := ResolveFileID(fileID)
	if err != nil {
		return nil, err
	}
	fileID = resolvedID
	var file *slackapi.File
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
