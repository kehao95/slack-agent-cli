package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFilesListCLIForwardsFiltersAndPages(t *testing.T) {
	oldConfig, oldTransport := cfgFile, http.DefaultTransport
	t.Cleanup(func() { cfgFile = oldConfig; http.DefaultTransport = oldTransport })
	cfgFile = setupValidConfig(t)
	t.Setenv("SLACK_TEAM_ID", "T123")
	t.Setenv("SLACK_CLI_READ_ONLY", "true")
	t.Setenv("HOME", t.TempDir())
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprintf("all=%t", all), func(t *testing.T) {
			calls := 0
			http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Path != "/api/files.list" {
					t.Errorf("unexpected method %s", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				for name, want := range map[string]string{"types": "images,pdfs", "count": "2", "page": fmt.Sprint(calls + 1), "user": "U123", "channel": "C123", "team_id": "T123"} {
					if got := r.Form.Get(name); got != want {
						t.Errorf("%s=%q, want %q", name, got, want)
					}
				}
				files := `[{"id":"F3"},{"id":"F4"}]`
				if calls > 1 {
					files = `[{"id":"F5"}]`
				}
				body := fmt.Sprintf(`{"ok":true,"files":%s,"paging":{"count":2,"total":5,"page":%d,"pages":3}}`, files, calls+1)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			command := newFilesListCommand()
			command.SetContext(context.Background())
			args := []string{"--user", "U123", "--channel", "C123", "--type", "images,pdfs", "--limit", "2", "--page", "2"}
			if all {
				args = append(args, "--all")
			}
			command.SetArgs(args)
			result := captureSearchJSON(t, command.Execute)
			wantCalls, wantFiles := 1, 2
			if all {
				wantCalls, wantFiles = 2, 3
			}
			if calls != wantCalls || len(result["files"].([]interface{})) != wantFiles || result["pages_fetched"] != float64(wantCalls) || result["has_more"] != !all {
				t.Fatalf("calls=%d result=%v", calls, result)
			}
			if !all && result["next_page"] != float64(3) {
				t.Fatalf("missing continuation: %v", result)
			}
		})
	}
}

func TestFilesListCLIRejectsInvalidInputBeforeNetwork(t *testing.T) {
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
		t.Errorf("invalid input made a request to %s", r.URL.Path)
		return nil, fmt.Errorf("unexpected network request")
	})
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--cursor", "NEXT"}, "unknown flag: --cursor"},
		{[]string{"--page", "0"}, "--page must be at least 1"},
		{[]string{"--limit", "0"}, "--limit must be between 1 and 1000"},
		{[]string{"--limit", "1001"}, "--limit must be between 1 and 1000"},
		{[]string{"--type", "canavs"}, "unsupported file type"},
		{[]string{"--type", "text"}, "unsupported file type"},
		{[]string{"--type", "all,canvas"}, "cannot be combined"},
		{[]string{"--max-retries", "-1"}, "cannot be negative"},
		{[]string{"--page-delay", "-1s"}, "cannot be negative"},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			command := newFilesListCommand()
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs(tt.args)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
}
