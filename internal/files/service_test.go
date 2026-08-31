package files

import (
	"bytes"
	"context"
	"io"
	"testing"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

type mockClient struct {
	files       []slackapi.File
	cursor      string
	info        *slackapi.File
	downloadURL string
	deleted     string
	public      bool
}

func (m *mockClient) UploadLocalFile(context.Context, string, appslack.UploadFileOptions) (*appslack.UploadFileResult, error) {
	return nil, nil
}
func (m *mockClient) ListFiles(_ context.Context, _ slackapi.ListFilesParameters) ([]slackapi.File, string, error) {
	return m.files, m.cursor, nil
}
func (m *mockClient) GetFileInfo(context.Context, string) (*slackapi.File, error) { return m.info, nil }
func (m *mockClient) DownloadFile(_ context.Context, url string, w io.Writer) error {
	m.downloadURL = url
	_, err := w.Write([]byte("content"))
	return err
}
func (m *mockClient) DeleteFile(_ context.Context, id string) error { m.deleted = id; return nil }
func (m *mockClient) ShareFilePublicURL(context.Context, string) (*slackapi.File, error) {
	m.public = true
	return &slackapi.File{PermalinkPublic: "https://public"}, nil
}
func (m *mockClient) RevokeFilePublicURL(context.Context, string) (*slackapi.File, error) {
	m.public = false
	return &slackapi.File{}, nil
}

func TestServiceListAndDownload(t *testing.T) {
	mock := &mockClient{files: []slackapi.File{{ID: "F1"}}, cursor: "next", info: &slackapi.File{ID: "F1", URLPrivateDownload: "https://download"}}
	service := NewService(mock)
	list, err := service.List(context.Background(), ListParams{})
	if err != nil || len(list.Files) != 1 || list.NextCursor != "next" {
		t.Fatalf("unexpected list: %#v err=%v", list, err)
	}
	var output bytes.Buffer
	file, err := service.Download(context.Background(), "F1", &output)
	if err != nil || file.ID != "F1" || output.String() != "content" || mock.downloadURL != "https://download" {
		t.Fatalf("unexpected download: file=%#v body=%q url=%q err=%v", file, output.String(), mock.downloadURL, err)
	}
}

func TestServiceMutations(t *testing.T) {
	mock := &mockClient{}
	service := NewService(mock)
	deleted, err := service.Delete(context.Background(), "F1")
	if err != nil || deleted.Action != "delete" || mock.deleted != "F1" {
		t.Fatalf("unexpected delete: %#v err=%v", deleted, err)
	}
	shared, err := service.SetPublic(context.Background(), "F1", true)
	if err != nil || !mock.public || shared.PermalinkPublic == "" {
		t.Fatalf("unexpected public result: %#v err=%v", shared, err)
	}
	revoked, err := service.SetPublic(context.Background(), "F1", false)
	if err != nil || mock.public || revoked.Action != "revoke-public" {
		t.Fatalf("unexpected revoke: %#v err=%v", revoked, err)
	}
}
