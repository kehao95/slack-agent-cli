package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/eventstore"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

func TestEventUserFiltersShareReferenceContract(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/users.list" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"members":[{"id":"U123","name":"will"}]}`)
	}))
	defer server.Close()
	for _, family := range []string{"query", "claim"} {
		for _, tt := range []struct {
			ref, want       string
			client, wantErr bool
		}{
			{ref: "U123", want: "U123"}, {ref: "<@W123>", want: "W123"},
			{ref: "@WiLl", want: "U123", client: true},
			{ref: "@will", wantErr: true}, {ref: "will", wantErr: true},
			{ref: "u123", wantErr: true}, {ref: "<@U123", wantErr: true},
		} {
			t.Run(family+"/"+tt.ref, func(t *testing.T) {
				command := &cobra.Command{Use: "test"}
				if family == "query" {
					addEventQueryFlags(command, false)
				} else {
					addEventClaimFlags(command)
				}
				if err := command.Flags().Set("user", tt.ref); err != nil {
					t.Fatal(err)
				}
				cmdCtx := &CommandContext{Ctx: context.Background()}
				if tt.client {
					cmdCtx.Client = appslack.New("xoxp-test", slackapi.OptionAPIURL(server.URL+"/"))
				}
				before := calls
				var filter eventstore.Filter
				var err error
				if family == "query" {
					filter, err = buildEventQueryFilter(command, cmdCtx, nil, false)
				} else {
					filter, err = buildEventClaimFilter(command, cmdCtx)
				}
				if (err != nil) != tt.wantErr {
					t.Fatalf("err=%v, wantErr=%t", err, tt.wantErr)
				}
				if !tt.wantErr && filter.UserID != tt.want {
					t.Fatalf("user=%q, want %q", filter.UserID, tt.want)
				}
				wantCalls := 0
				if tt.client {
					wantCalls = 1
				}
				if calls-before != wantCalls {
					t.Fatalf("got %d network calls, want %d", calls-before, wantCalls)
				}
			})
		}
	}
}

type userReferenceTransport func(*http.Request) (*http.Response, error)

func (f userReferenceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBatchUserCommandsShareOneLookup(t *testing.T) {
	oldConfig, oldTransport := cfgFile, http.DefaultTransport
	t.Cleanup(func() { cfgFile = oldConfig; http.DefaultTransport = oldTransport })
	cfgFile = setupValidConfig(t)
	t.Setenv("SLACK_TEAM_ID", "TIDENTITYTEST")
	t.Setenv("HOME", t.TempDir())

	for _, tt := range []struct {
		name, method string
		run          func(*cobra.Command, []string) error
	}{
		{"members", "usergroups.users.update", runUsergroupsMembersSet},
		{"invite", "conversations.invite", runConversationsInvite},
		{"open", "conversations.open", runConversationsOpen},
	} {
		for _, mode := range []string{"usernames", "ids", "invalid"} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				var paths []string
				http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
					paths = append(paths, r.URL.Path)
					body := ""
					switch r.URL.Path {
					case "/api/users.list":
						body = `{"ok":true,"members":[{"id":"U1","name":"will"},{"id":"U2","name":"alice","profile":{"display_name":"Will"}}]}`
					case "/api/" + tt.method:
						if err := r.ParseForm(); err != nil {
							t.Error(err)
						}
						if users := r.Form.Get("users"); users != "U1,U2" {
							t.Errorf("users=%q, want U1,U2", users)
						}
						body = `{"ok":true,"channel":{"id":"C123"},"usergroup":{"id":"S123","users":["U1","U2"]}}`
					default:
						t.Errorf("unexpected request %s", r.URL.Path)
						body = `{"ok":false,"error":"unexpected_request"}`
					}
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})
				refs := []string{"@will", "@alice", "@will"}
				if mode == "ids" {
					refs = []string{"<@U1>", "U2", "U1"}
				}
				if mode == "invalid" {
					refs = []string{"@will", "alice"}
				}
				command := &cobra.Command{Use: "test"}
				command.SetContext(context.Background())
				channel := "C123"
				if tt.name == "open" {
					channel = ""
				}
				command.Flags().String("channel", channel, "")
				command.Flags().String("group", "S123", "")
				command.Flags().String("members", strings.Join(refs, ","), "")
				command.Flags().StringSlice("users", refs, "")
				err := tt.run(command, nil)
				if mode == "invalid" {
					if err == nil || !strings.Contains(err.Error(), "invalid user reference") {
						t.Fatalf("expected invalid user reference, got %v", err)
					}
					if len(paths) != 0 {
						t.Fatalf("invalid batch made requests: %v", paths)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				expected := []string{"/api/" + tt.method}
				if mode == "usernames" {
					expected = append([]string{"/api/users.list"}, expected...)
				}
				if !reflect.DeepEqual(paths, expected) {
					t.Fatalf("paths=%v, want %v", paths, expected)
				}
			})
		}
	}
}
