package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
)

func TestListUsesNumberedPagesAndFilters(t *testing.T) {
	for _, tt := range []struct {
		name      string
		start     int
		all       bool
		empty     bool
		wantIDs   []string
		wantPages []int
		wantNext  int
	}{
		{name: "first", start: 1, wantIDs: []string{"F1", "F2"}, wantPages: []int{1}, wantNext: 2},
		{name: "later", start: 2, wantIDs: []string{"F3", "F4"}, wantPages: []int{2}, wantNext: 3},
		{name: "last", start: 3, wantIDs: []string{"F5"}, wantPages: []int{3}},
		{name: "all", start: 1, all: true, wantIDs: []string{"F1", "F2", "F3", "F4", "F5"}, wantPages: []int{1, 2, 3}},
		{name: "all remaining", start: 2, all: true, wantIDs: []string{"F3", "F4", "F5"}, wantPages: []int{2, 3}},
		{name: "empty", start: 1, empty: true, wantIDs: []string{}, wantPages: []int{1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SLACK_CLI_READ_ONLY", "true")
			var pages []int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/files.list" {
					t.Errorf("unexpected method %s", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				for name, want := range map[string]string{"count": "2", "types": "canvas", "user": "U123", "channel": "C123", "team_id": "T123"} {
					if got := r.Form.Get(name); got != want {
						t.Errorf("%s=%q, want %q", name, got, want)
					}
				}
				if r.Form.Has("limit") || r.Form.Has("cursor") {
					t.Error("files.list received cursor parameters")
				}
				page := 1 // Slack's default; the SDK omits page=1.
				if value := r.Form.Get("page"); value != "" {
					if _, err := fmt.Sscan(value, &page); err != nil {
						t.Error(err)
					}
				}
				pages = append(pages, page)
				var files []slackapi.File
				paging := slackapi.Paging{Count: 2, Total: 5, Page: page, Pages: 3}
				if tt.empty {
					paging.Total, paging.Pages = 0, 0
				} else {
					for n := (page-1)*2 + 1; n <= page*2 && n <= 5; n++ {
						files = append(files, slackapi.File{ID: fmt.Sprintf("F%d", n), Filetype: "quip"})
					}
				}
				writeListResponse(t, w, files, &paging)
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := NewService(client).List(context.Background(), ListParams{
				Limit: 2, Page: tt.start, All: tt.all, Types: "canvas", User: "U123", Channel: "C123", TeamID: "T123",
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, file := range result.Files {
				ids = append(ids, file.ID)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) || !reflect.DeepEqual(pages, tt.wantPages) {
				t.Fatalf("files=%v pages=%v; want files=%v pages=%v", ids, pages, tt.wantIDs, tt.wantPages)
			}
			if !result.OK || result.NextPage != tt.wantNext || result.HasMore != (tt.wantNext != 0) || result.PagesFetched != len(tt.wantPages) || result.Paging.Page != tt.wantPages[len(tt.wantPages)-1] {
				t.Fatalf("unexpected continuation: %+v", result)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "next_cursor") || (tt.empty && !strings.Contains(string(encoded), `"files":[]`)) {
				t.Fatalf("unexpected JSON: %s", encoded)
			}
			if tt.wantNext == 0 && strings.Contains(string(encoded), `"next_page"`) {
				t.Fatalf("completed listing advertises continuation: %s", encoded)
			}
		})
	}
}

func TestListRejectsBrokenPaging(t *testing.T) {
	for _, tt := range []struct {
		name   string
		paging *slackapi.Paging
		files  []slackapi.File
		want   string
	}{
		{name: "missing", files: []slackapi.File{{ID: "F1"}}, want: "invalid file paging"},
		{name: "wrong page", paging: &slackapi.Paging{Count: 2, Total: 5, Page: 1, Pages: 3}, want: "invalid file paging"},
		{name: "missing pages", paging: &slackapi.Paging{Count: 2, Total: 5, Page: 2}, want: "without page count"},
		{name: "negative total", paging: &slackapi.Paging{Count: 2, Total: -1, Page: 2, Pages: 3}, want: "invalid file paging"},
		{name: "oversized", paging: &slackapi.Paging{Count: 100, Total: 5, Page: 2, Pages: 3}, files: []slackapi.File{{ID: "F1"}, {ID: "F2"}, {ID: "F3"}}, want: "exceeding requested limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				writeListResponse(t, w, tt.files, tt.paging)
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := NewService(client).List(context.Background(), ListParams{Limit: 2, Page: 2, All: true})
			if result != nil || err == nil || !strings.Contains(err.Error(), tt.want) || calls != 1 {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestListStopsOnRepeatedPages(t *testing.T) {
	for _, repeatNumber := range []bool{false, true} {
		t.Run(fmt.Sprintf("repeated page number=%t", repeatNumber), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				page := calls
				if repeatNumber {
					page = 1
				}
				writeListResponse(t, w, []slackapi.File{{ID: "F1"}}, &slackapi.Paging{Count: 2, Total: 20, Page: page, Pages: 10})
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			result, err := NewService(client).List(context.Background(), ListParams{Limit: 2, All: true})
			if result != nil || err == nil || calls != 2 {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestListHonorsRateLimitAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		retries    int
		cancel     bool
		wantCalls  int
		wantFailed bool
	}{
		{name: "retry", retries: 1, wantCalls: 2},
		{name: "retry exhausted", wantCalls: 1, wantFailed: true},
		{name: "cancel waiting", retries: 1, cancel: true, wantCalls: 1, wantFailed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("page") != "2" {
					t.Error("retry advanced the requested page")
				}
				if calls == 1 {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				writeListResponse(t, w, []slackapi.File{{ID: "F3"}}, &slackapi.Paging{Count: 2, Total: 3, Page: 2, Pages: 2})
			}))
			defer server.Close()
			client := appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
			ctx := context.Background()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			start := time.Now()
			_, err := NewService(client).List(ctx, ListParams{Limit: 2, Page: 2, MaxRetries: tt.retries})
			elapsed := time.Since(start)
			if (err != nil) != tt.wantFailed || calls != tt.wantCalls {
				t.Fatalf("err=%v calls=%d; want failure=%t calls=%d", err, calls, tt.wantFailed, tt.wantCalls)
			}
			if !tt.wantFailed && elapsed < time.Second {
				t.Fatalf("retried before Retry-After: %v", elapsed)
			}
			if tt.cancel && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if tt.retries == 0 {
				var limited *slackapi.RateLimitedError
				if !errors.As(err, &limited) {
					t.Fatalf("lost rate-limit error: %v", err)
				}
			}
		})
	}
}

func TestNormalizeListTypes(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"", "all"}, {" all ", "all"}, {"canvas", "canvas"}, {"quip", "quip"},
		{"images, PDFs,images", "images,pdfs"}, {"spaces,snippets,gdocs,zips", "spaces,snippets,gdocs,zips"},
	} {
		got, err := NormalizeListTypes(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("NormalizeListTypes(%q)=%q,%v; want %q", tt.input, got, err, tt.want)
		}
	}
	for _, value := range []string{"text", "canavs", "png", "all,canvas", "canvas,", "canvas,,images"} {
		if _, err := NormalizeListTypes(value); err == nil {
			t.Errorf("NormalizeListTypes(%q) should reject a potentially ignored filter", value)
		}
	}
}

func TestListValidatesBeforeNetwork(t *testing.T) {
	// No client is supplied: each invalid input must fail before calling Slack.
	for _, params := range []ListParams{
		{Limit: -1}, {Limit: 1001}, {Page: -1}, {Types: "canavs"}, {MaxRetries: -1}, {PageDelay: -1},
	} {
		if _, err := NewService(nil).List(context.Background(), params); err == nil {
			t.Errorf("List(%+v) should fail", params)
		}
	}
}

func TestListHumanOutputDisclosesContinuation(t *testing.T) {
	result := &ListResult{Paging: slackapi.Paging{Count: 2, Total: 5, Page: 2, Pages: 3}, PagesFetched: 1, HasMore: true, NextPage: 3}
	text := strings.Join(result.Lines(), "\n")
	for _, want := range []string{"No files found on the requested pages", "last page: 2 of 3", "total files: 5", "--page 3 --limit 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("human output omits %q: %s", want, text)
		}
	}
}

func writeListResponse(t *testing.T, w http.ResponseWriter, files []slackapi.File, paging *slackapi.Paging) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"ok": true, "files": files}
	if paging != nil {
		response["paging"] = paging
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Error(err)
	}
}
