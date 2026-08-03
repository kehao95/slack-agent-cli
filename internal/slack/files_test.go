package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestUploadImageValidation(t *testing.T) {
	client := New("xoxp-test-token")

	tests := []struct {
		name    string
		channel string
		path    string
		want    string
	}{
		{name: "missing channel", channel: "", path: "image.png", want: "channel is required"},
		{name: "missing path", channel: "C123", path: "", want: "image path is required"},
		{name: "missing file", channel: "C123", path: "/does/not/exist.png", want: "open image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.UploadImage(context.Background(), tt.channel, tt.path, UploadImageOptions{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	emptyPath := filepath.Join(t.TempDir(), "empty.png")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("create empty image: %v", err)
	}
	_, err := client.UploadImage(context.Background(), "C123", emptyPath, UploadImageOptions{})
	if err == nil || !strings.Contains(err.Error(), "image file is empty") {
		t.Fatalf("expected empty image error, got %v", err)
	}
}

func TestUploadImageUsesExternalUploadFlow(t *testing.T) {
	imageData := []byte("fake image bytes")
	imagePath := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(imagePath, imageData, 0o600); err != nil {
		t.Fatalf("create image: %v", err)
	}

	var gotUpload bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files.getUploadURLExternal":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for upload URL, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse upload URL form: %v", err)
			}
			if got := r.Form.Get("filename"); got != "screenshot.png" {
				t.Errorf("expected filename screenshot.png, got %q", got)
			}
			if got := r.Form.Get("length"); got != strconv.Itoa(len(imageData)) {
				t.Errorf("expected length %d, got %q", len(imageData), got)
			}
			if got := r.Form.Get("alt_txt"); got != "A screenshot" {
				t.Errorf("expected alt text, got %q", got)
			}
			writeJSON(t, w, map[string]any{
				"ok":         true,
				"upload_url": server.URL + "/upload",
				"file_id":    "F123IMAGE",
			})

		case "/upload":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for file upload, got %s", r.Method)
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("read uploaded file: %v", err)
			}
			defer file.Close()
			if header.Filename != "screenshot.png" {
				t.Errorf("expected uploaded filename screenshot.png, got %q", header.Filename)
			}
			got, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read uploaded bytes: %v", err)
			}
			if string(got) != string(imageData) {
				t.Errorf("uploaded bytes differ: got %q", got)
			}
			gotUpload = true
			writeJSON(t, w, map[string]any{})

		case "/files.completeUploadExternal":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for completion, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse completion form: %v", err)
			}
			if got := r.Form.Get("channel_id"); got != "C123" {
				t.Errorf("expected channel C123, got %q", got)
			}
			if got := r.Form.Get("thread_ts"); got != "1700000000.000100" {
				t.Errorf("expected thread timestamp, got %q", got)
			}
			if got := r.Form.Get("initial_comment"); got != "See this" {
				t.Errorf("expected initial comment, got %q", got)
			}
			var files []slackapi.FileSummary
			if err := json.Unmarshal([]byte(r.Form.Get("files")), &files); err != nil {
				t.Fatalf("parse files payload: %v", err)
			}
			if len(files) != 1 || files[0].ID != "F123IMAGE" {
				t.Fatalf("unexpected files payload: %#v", files)
			}
			writeJSON(t, w, map[string]any{
				"ok":    true,
				"files": []slackapi.FileSummary{{ID: "F123IMAGE", Title: "screenshot.png"}},
			})

		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New("xoxp-test-token", slackapi.OptionAPIURL(server.URL+"/"))
	result, err := client.UploadImage(context.Background(), "C123", imagePath, UploadImageOptions{
		ThreadTS:       "1700000000.000100",
		InitialComment: "See this",
		AltText:        "A screenshot",
	})
	if err != nil {
		t.Fatalf("upload image: %v", err)
	}
	if !gotUpload {
		t.Fatal("expected the file upload step to run")
	}
	if result.FileID != "F123IMAGE" || result.Filename != "screenshot.png" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Channel != "C123" || result.ThreadTS != "1700000000.000100" {
		t.Fatalf("unexpected destination: %#v", result)
	}
}

func TestUploadImageResultLines(t *testing.T) {
	result := &UploadImageResult{
		Channel:  "#general",
		FileID:   "F123IMAGE",
		Filename: "screenshot.png",
		ThreadTS: "1700000000.000100",
	}
	lines := result.Lines()
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	for _, want := range []string{"Image sent successfully", "#general", "F123IMAGE", "screenshot.png", "1700000000.000100"} {
		found := false
		for _, line := range lines {
			if strings.Contains(line, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected output to contain %q, got %v", want, lines)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}
