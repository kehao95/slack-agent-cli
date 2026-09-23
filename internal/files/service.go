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
	"time"
	"unicode/utf8"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type Client interface {
	UploadLocalFile(context.Context, string, appslack.UploadFileOptions) (*appslack.UploadFileResult, error)
	ListFiles(context.Context, slackapi.GetFilesParameters) ([]slackapi.File, *slackapi.Paging, error)
	GetFileInfo(context.Context, string) (*slackapi.File, error)
	DownloadFile(context.Context, string, io.Writer) error
	DeleteFile(context.Context, string) error
	ShareFilePublicURL(context.Context, string) (*slackapi.File, error)
	RevokeFilePublicURL(context.Context, string) (*slackapi.File, error)
}

type Service struct{ client Client }

func NewService(client Client) *Service { return &Service{client: client} }

type ListParams struct {
	Limit      int
	Page       int
	All        bool
	User       string
	Channel    string
	TeamID     string
	Types      string
	MaxRetries int
	PageDelay  time.Duration
}

type ListResult struct {
	OK           bool            `json:"ok"`
	Files        []slackapi.File `json:"files"`
	Paging       slackapi.Paging `json:"paging"`
	PagesFetched int             `json:"pages_fetched"`
	HasMore      bool            `json:"has_more"`
	NextPage     int             `json:"next_page,omitempty"`
}

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.Limit < 1 || params.Limit > 1000 {
		return nil, fmt.Errorf("file limit must be between 1 and 1000")
	}
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Page < 1 {
		return nil, fmt.Errorf("file page must be at least 1")
	}
	if params.MaxRetries < 0 || params.PageDelay < 0 {
		return nil, fmt.Errorf("file max retries and page delay cannot be negative")
	}
	types, err := NormalizeListTypes(params.Types)
	if err != nil {
		return nil, err
	}
	params.Types = types
	result := &ListResult{OK: true, Files: []slackapi.File{}}
	seen := make(map[string]bool)
	for page := params.Page; ; page++ {
		current, err := appslack.RetryRateLimited(ctx, params.MaxRetries, func() (*ListResult, error) {
			items, paging, err := s.client.ListFiles(ctx, slackapi.GetFilesParameters{
				Count: params.Limit, Page: page, User: params.User, Channel: params.Channel, TeamID: params.TeamID, Types: params.Types,
			})
			if err != nil {
				return nil, err
			}
			if paging == nil || paging.Page != page || paging.Count < 1 || paging.Total < 0 || paging.Pages < 0 {
				return nil, fmt.Errorf("Slack returned invalid file paging for requested page %d", page)
			}
			if len(items) > params.Limit {
				return nil, fmt.Errorf("Slack returned %d files, exceeding requested limit %d", len(items), params.Limit)
			}
			if paging.Total > 0 && paging.Pages < 1 {
				return nil, fmt.Errorf("Slack returned file totals without page count")
			}
			return &ListResult{Files: items, Paging: *paging}, nil
		})
		if err != nil {
			return nil, err
		}
		for _, file := range current.Files {
			if seen[file.ID] {
				return nil, fmt.Errorf("Slack returned repeated file %s on page %d; the listing changed or pagination did not advance", file.ID, page)
			}
			seen[file.ID] = true
		}
		result.Files = append(result.Files, current.Files...)
		result.Paging = current.Paging
		result.PagesFetched++
		result.HasMore = page < current.Paging.Pages
		result.NextPage = 0
		if result.HasMore {
			result.NextPage = page + 1
		}
		if !params.All || !result.HasMore {
			return result, nil
		}
		if err := appslack.WaitContext(ctx, params.PageDelay); err != nil {
			return nil, err
		}
	}
}

// NormalizeListTypes accepts Slack's documented filter groups and verified
// Canvas aliases. Slack silently ignores unknown types and returns all files.
func NormalizeListTypes(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "all", nil
	}
	var types []string
	seen := make(map[string]bool)
	for _, value := range strings.Split(value, ",") {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "all", "spaces", "snippets", "images", "gdocs", "zips", "pdfs", "canvas", "quip":
		default:
			return "", fmt.Errorf("unsupported file type %q: use all, spaces, snippets, images, gdocs, zips, pdfs, canvas, or quip", value)
		}
		if !seen[value] {
			types = append(types, value)
			seen[value] = true
		}
	}
	if seen["all"] && len(types) > 1 {
		return "", fmt.Errorf("file type all cannot be combined with narrower filters")
	}
	return strings.Join(types, ","), nil
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
	lines := []string{fmt.Sprintf("Files (%d)", len(r.Files))}
	if len(r.Files) == 0 {
		lines = []string{"No files found on the requested pages."}
	}
	for _, file := range r.Files {
		lines = append(lines, fmt.Sprintf("%s  %s  %d bytes", file.ID, firstNonEmpty(file.Title, file.Name), file.Size))
	}
	lines = append(lines, fmt.Sprintf("Pages fetched: %d; last page: %d of %d; page size: %d; total files: %d", r.PagesFetched, r.Paging.Page, r.Paging.Pages, r.Paging.Count, r.Paging.Total))
	if r.HasMore {
		lines = append(lines, fmt.Sprintf("More files available; continue with the same filters and --page %d --limit %d", r.NextPage, r.Paging.Count))
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
