package files

import (
	"context"
	"io"
	"strings"
	"testing"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

func TestResolveFileIDDocumentedURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "permalink", url: "https://example.slack.com/files/U123/F123ABC/report.txt", want: "F123ABC"},
		{name: "private", url: "https://files.slack.com/files-pri/T123-F123ABC/report.txt", want: "F123ABC"},
		{name: "client", url: "https://app.slack.com/client/files/T123/F123ABC/report.txt", want: "F123ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFileID(tt.url)
			if err != nil {
				t.Fatalf("ResolveFileID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveFileID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveFileIDDoesNotTreatURLPrefixesAsIDs(t *testing.T) {
	for _, reference := range []string{
		"https://files.slack.com/files-pri/T123-F123ABC/report.txt",
		"https://example.slack.com/files/U123/F123ABC/report.txt",
		"https://example.slack.com/files/U123/title.txt",
	} {
		if got, err := ResolveFileID(reference); err != nil && got != "" {
			t.Fatalf("ResolveFileID(%q) returned ID %q with error %v", reference, got, err)
		}
	}
	if got, err := ResolveFileID("https://example.slack.com/files-pri/report.txt"); err == nil || got != "" {
		t.Fatalf("ResolveFileID() = %q, %v; want rejection", got, err)
	}
}

type largeContentClient struct{}

func (largeContentClient) UploadLocalFile(context.Context, string, appslack.UploadFileOptions) (*appslack.UploadFileResult, error) {
	return nil, nil
}
func (largeContentClient) ListFiles(context.Context, slackapi.ListFilesParameters) ([]slackapi.File, string, error) {
	return nil, "", nil
}
func (largeContentClient) GetFileInfo(context.Context, string) (*slackapi.File, error) {
	return &slackapi.File{ID: "F123", URLPrivateDownload: "https://files.slack.com/files-pri/T123-F123/content"}, nil
}
func (largeContentClient) DownloadFile(_ context.Context, _ string, writer io.Writer) error {
	_, err := io.Copy(writer, strings.NewReader(strings.Repeat("x", 16<<20+1)))
	return err
}
func (largeContentClient) DeleteFile(context.Context, string) error { return nil }
func (largeContentClient) ShareFilePublicURL(context.Context, string) (*slackapi.File, error) {
	return &slackapi.File{}, nil
}
func (largeContentClient) RevokeFilePublicURL(context.Context, string) (*slackapi.File, error) {
	return &slackapi.File{}, nil
}

func TestReadBytesStopsAtLimit(t *testing.T) {
	_, _, err := NewService(largeContentClient{}).ReadBytes(context.Background(), "F123")
	if err == nil || !strings.Contains(err.Error(), "16 MiB read limit") {
		t.Fatalf("ReadBytes() error = %v, want bounded-read error", err)
	}
}
