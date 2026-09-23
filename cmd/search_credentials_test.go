package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/config"
	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/spf13/cobra"
)

func setupSearchCredentials(t *testing.T, role, token, cookie string) {
	t.Helper()
	old := cfgFile
	t.Cleanup(func() { cfgFile = old })
	clearAuthEnvForTest(t)
	t.Setenv("SLACK_CLIENT_COOKIE", "")
	t.Setenv("SLACK_TEAM_ID", "")
	t.Setenv("HOME", t.TempDir())
	cfgFile = filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultConfig()
	cfg.UserToken, cfg.BotToken, cfg.Role, cfg.Cookie = token, "xoxb-stored-bot", role, cookie
	if _, err := config.Save(cfgFile, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestOrdinarySearchRejectsBotBeforeAnyNetwork(t *testing.T) {
	for _, readOnly := range []string{"false", "true"} {
		t.Run("read_only="+readOnly, func(t *testing.T) {
			t.Setenv("SLACK_CLI_READ_ONLY", readOnly)

			for _, identity := range []struct{ role, token string }{
				{"bot", "xoxp-stored-user"}, {"user", "xoxb-wrong-slot"},
			} {
				t.Run(identity.role+"/"+identity.token[:5], func(t *testing.T) {
					setupSearchCredentials(t, identity.role, identity.token, "")
					calls := 0
					old := http.DefaultTransport
					t.Cleanup(func() { http.DefaultTransport = old })
					http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
						calls++
						return nil, fmt.Errorf("unexpected network request to %s", r.URL.Path)
					})
					for _, surface := range []string{"all", "messages", "files", "legacy"} {
						command := newSearchResourceCommand("messages", "test")
						command.SetContext(context.Background())
						_ = command.Flags().Set("query", "deployment")
						var err error
						if surface == "legacy" {
							err = runMessagesSearch(command, nil)
						} else {
							err = runUnifiedSearch(command, surface)
						}
						if err == nil || !strings.Contains(err.Error(), "SLACK_CLI_ROLE=user") || !strings.Contains(err.Error(), "search:read") {
							t.Fatalf("%s: expected actionable user-only error, got %v", surface, err)
						}
						if strings.Contains(err.Error(), identity.token) || strings.Contains(err.Error(), "xoxb-stored-bot") {
							t.Fatalf("credential in error: %v", err)
						}
						var coded *cerrors.ErrorWithExitCode
						if !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitAuth {
							t.Fatalf("expected auth exit, got %v", err)
						}
					}
					for _, method := range []string{"search.all", "search.messages", "search.files"} {
						if err := requireOrdinarySearchUser(method); err == nil {
							t.Fatalf("raw method preflight allowed %s", method)
						}
					}
					if calls != 0 {
						t.Fatalf("made %d calls before rejecting bot", calls)
					}
				})
			}
		})
	}

}

func captureSearchJSON(t *testing.T, run func() error) map[string]interface{} {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = old; file.Close() }()
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOrdinarySearchActiveUserAndCookiePreserved(t *testing.T) {
	for _, readOnly := range []string{"false", "true"} {
		t.Run("read_only="+readOnly, func(t *testing.T) {
			t.Setenv("SLACK_CLI_READ_ONLY", readOnly)

			for _, token := range []string{"xoxp-active-user", "xoxc-active-client"} {
				t.Run(token[:4], func(t *testing.T) {
					cookie := ""
					if strings.HasPrefix(token, "xoxc-") {
						cookie = "xoxd-active-cookie"
					}
					setupSearchCredentials(t, "user", token, cookie)
					t.Setenv("SLACK_TEAM_ID", "TSEARCHTEST")
					old := http.DefaultTransport
					t.Cleanup(func() { http.DefaultTransport = old })
					for _, surface := range []string{"all", "messages", "files", "legacy"} {
						t.Run(surface, func(t *testing.T) {
							var pages []string
							http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
								method := surface
								if method == "legacy" {
									method = "messages"
								}
								if r.URL.Path != "/api/search."+method {
									t.Errorf("unexpected endpoint %s", r.URL.Path)
								}
								if err := r.ParseForm(); err != nil {
									t.Error(err)
								}
								if r.Form.Get("token") != token && r.Header.Get("Authorization") != "Bearer "+token {
									t.Error("request did not use selected user credential")
								}
								if strings.Contains(r.Header.Get("Authorization"), "stored-bot") || r.Form.Get("token") == "xoxb-stored-bot" {
									t.Error("request used bot identity")
								}
								if cookie != "" {
									got, err := r.Cookie("d")
									if err != nil || got.Value != cookie {
										t.Errorf("client cookie missing: %v", err)
									}
								}
								page := r.Form.Get("page")
								if page == "" {
									page = "1"
								} // The SDK omits Slack's default first page.
								pages = append(pages, page)
								body := fmt.Sprintf(`{"ok":true,"messages":{"total":2,"paging":{"pages":2},"matches":[{"type":"message","user":"U123","text":"hi <@U456>","ts":"%s.0","permalink":"https://example.slack.com/archives/C123/p%s000000","channel":{"id":"C123","name":"general"},"attachments":[{"text":"Synthetic alert details","title_link":"https://example.test/alert"}],"blocks":[{"type":"section","text":{"type":"mrkdwn","text":"Synthetic block details"}}]}]},"files":{"total":2,"paging":{"pages":2},"matches":[{"id":"F%s","title":"Report"}]}}`, page, page, page)
								return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
							})
							command := newSearchResourceCommand("messages", "test")
							command.SetContext(context.Background())
							_ = command.Flags().Set("query", "deployment")
							_ = command.Flags().Set("raw-json", "true")
							_ = command.Flags().Set("all", "true")
							output := captureSearchJSON(t, func() error {
								if surface == "legacy" {
									return runMessagesSearch(command, nil)
								}
								return runUnifiedSearch(command, surface)
							})
							wantPages := []string{"1", "2"}
							if surface == "legacy" {
								wantPages = []string{"1"}
								if _, exists := output["kind"]; exists {
									t.Fatal("legacy envelope gained unified fields")
								}
							}
							if !reflect.DeepEqual(pages, wantPages) {
								t.Fatalf("pages=%v, want %v", pages, wantPages)
							}
							if output["has_more"] != (surface == "legacy") || output["page_count"] != float64(2) {
								t.Fatalf("pagination missing from %s output: %v", surface, output)
							}
							if surface == "legacy" && (output["next_page"] != float64(2) || output["returned_count"] != float64(1)) {
								t.Fatalf("legacy output hides partial search: %v", output)
							}
							encoded, _ := json.Marshal(output)
							if strings.Contains(string(encoded), token) || (cookie != "" && strings.Contains(string(encoded), cookie)) {
								t.Fatal("credential leaked to output")
							}
							if surface != "files" {
								matches := output["messages"].(map[string]interface{})["matches"].([]interface{})
								first := matches[0].(map[string]interface{})
								if first["user"] != "U123" || first["text"] != "hi <@U456>" || first["permalink"] == "" {
									t.Fatalf("lost message identity/text/link: %v", first)
								}
								attachments, _ := first["attachments"].([]interface{})
								blocks, _ := first["blocks"].([]interface{})
								if len(attachments) != 1 || len(blocks) != 1 {
									t.Fatalf("%s lost structured content: %v", surface, first)
								}
								if len(matches) != len(wantPages) {
									t.Fatalf("lost pages: %v", matches)
								}
							}
						})
					}
				})
			}
		})
	}

}

func TestOrdinarySearchHelpExplainsIdentityAndScope(t *testing.T) {
	for _, command := range []*cobra.Command{searchAllCmd, searchMessagesCmd, searchFilesCmd, messagesSearchCmd} {
		if !strings.Contains(command.Long, "search:read") || !strings.Contains(command.Long, "SLACK_CLI_ROLE=user") {
			t.Errorf("incomplete search auth help for %s", command.Name())
		}
	}
}

func TestOrdinarySearchPermissionFailureIsNotEmptySuccess(t *testing.T) {
	setupSearchCredentials(t, "user", "xoxp-active-user", "")
	t.Setenv("SLACK_TEAM_ID", "TSEARCHTEST")
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, surface := range []string{"all", "messages", "files", "legacy"} {
		t.Run(surface, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":"missing_scope","needed":"search:read"}`)), Request: r}, nil
			})
			command := newSearchResourceCommand("messages", "test")
			command.SetContext(context.Background())
			_ = command.Flags().Set("query", "deployment")
			var err error
			if surface == "legacy" {
				err = runMessagesSearch(command, nil)
			} else {
				err = runUnifiedSearch(command, surface)
			}
			if err == nil || !strings.Contains(err.Error(), "missing_scope") {
				t.Fatalf("missing scope became successful search: %v", err)
			}
			if cerrors.ClassifySlackError(err) != cerrors.ExitPermission {
				t.Fatalf("wrong permission error classification: %v", err)
			}
			if calls != 1 {
				t.Fatalf("expected one search call, got %d", calls)
			}
		})
	}
}

func TestRawOrdinarySearchCredentialPolicy(t *testing.T) {
	for _, readOnly := range []string{"false", "true"} {
		t.Run("read_only="+readOnly, func(t *testing.T) {
			t.Setenv("SLACK_CLI_READ_ONLY", readOnly)
			for _, identity := range []struct {
				name, role, token, cookie string
				blocked                   bool
			}{
				{name: "bot-with-stored-user", role: "bot", token: "xoxp-stored-user", blocked: true},
				{name: "bot-in-user-slot", role: "user", token: "xoxb-wrong-slot", blocked: true},
				{name: "user", role: "user", token: "xoxp-active-user"},
				{name: "client-cookie", role: "user", token: "xoxc-active-client", cookie: "xoxd-active-cookie"},
			} {
				t.Run(identity.name, func(t *testing.T) {
					setupSearchCredentials(t, identity.role, identity.token, identity.cookie)
					if !identity.blocked {
						t.Setenv("SLACK_TEAM_ID", "TSEARCHTEST")
					}
					old := http.DefaultTransport
					t.Cleanup(func() { http.DefaultTransport = old })
					for _, method := range []string{"search.all", "search.messages", "search.files"} {
						t.Run(method, func(t *testing.T) {
							calls := 0
							http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
								calls++
								if r.URL.Path != "/api/"+method {
									t.Errorf("unexpected endpoint %s", r.URL.Path)
								}
								if r.Header.Get("Authorization") != "Bearer "+identity.token {
									t.Error("raw search used a different credential")
								}
								if identity.cookie != "" {
									cookie, err := r.Cookie("d")
									if err != nil || cookie.Value != identity.cookie {
										t.Errorf("client cookie missing: %v", err)
									}
								}
								if err := r.ParseForm(); err != nil {
									t.Error(err)
								}
								if r.Form.Get("query") != "deployment" || r.Form.Get("page") != "2" {
									t.Errorf("raw search request changed: %v", r.Form)
								}
								body := `{"ok":true,"messages":{"total":1,"paging":{"page":2,"pages":3},"matches":[{"user":"U123","text":"hi <@U456>","permalink":"https://example.slack.com/message"}]},"files":{"matches":[{"id":"F123"}]}}`
								return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
							})
							command := &cobra.Command{Use: "api"}
							command.SetContext(context.Background())
							command.Flags().String("data", "-", "")
							command.Flags().Int("max-retries", 0, "")
							input := bytes.NewBufferString(`{"query":"deployment","page":2}`)
							before := input.Len()
							command.SetIn(input)
							var stdout bytes.Buffer
							command.SetOut(&stdout)
							err := runAPI(command, []string{method})
							if identity.blocked {
								var coded *cerrors.ErrorWithExitCode
								if !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitAuth || !strings.Contains(err.Error(), "search:read") {
									t.Fatalf("expected actionable auth failure, got %v", err)
								}
								if calls != 0 || input.Len() != before || stdout.Len() != 0 {
									t.Fatalf("blocked raw search had side effects: calls=%d unread=%d stdout=%q", calls, input.Len(), stdout.String())
								}
								if strings.Contains(err.Error(), identity.token) {
									t.Fatalf("credential leaked: %v", err)
								}
								return
							}
							if err != nil {
								t.Fatal(err)
							}
							if calls != 1 {
								t.Fatalf("expected one allowed raw search call, got %d", calls)
							}
							var output map[string]interface{}
							if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
								t.Fatal(err)
							}
							messages := output["messages"].(map[string]interface{})
							match := messages["matches"].([]interface{})[0].(map[string]interface{})
							paging := messages["paging"].(map[string]interface{})
							if output["ok"] != true || match["user"] != "U123" || match["text"] != "hi <@U456>" || match["permalink"] == "" || paging["page"] != float64(2) || paging["pages"] != float64(3) {
								t.Fatalf("raw response lost data: %s", stdout.String())
							}
							if strings.Contains(stdout.String(), identity.token) || (identity.cookie != "" && strings.Contains(stdout.String(), identity.cookie)) {
								t.Fatal("raw search exposed credentials")
							}
						})
					}
				})
			}
		})
	}
}
