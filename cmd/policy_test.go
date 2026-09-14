package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/policy"
	"github.com/spf13/cobra"
)

// Exercise the actual Cobra tree and production exit-code handling in fresh
// processes. No real HTTP is possible, including auth.test during setup.
func TestPolicyCLIHelper(t *testing.T) {
	encoded := os.Getenv("SLK_TEST_POLICY_ARGS")
	if encoded == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(encoded), &args); err != nil {
		t.Fatal(err)
	}
	http.DefaultTransport = userReferenceTransport(func(r *http.Request) (*http.Response, error) {
		fmt.Fprintln(os.Stderr, "TEST_HTTP_REQUEST", r.URL.Path)
		body := `{"ok":true}`
		switch r.URL.Path {
		case "/api/auth.test":
			body = `{"ok":true,"team_id":"TTEST","user_id":"UTEST"}`
		case "/api/users.info":
			body = `{"ok":true,"user":{"id":"UTEST","name":"alice","profile":{"display_name":"Alice"}}}`
		case "/api/conversations.history":
			body = `{"ok":true,"messages":[],"has_more":false}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	rootCmd.SetArgs(args)
	Execute()
}

func runPolicyCLI(t *testing.T, setting string, args ...string) (int, string) {
	t.Helper()
	configPath := setupValidConfig(t)
	encoded, err := json.Marshal(append([]string{"--config", configPath}, args...))
	if err != nil {
		t.Fatal(err)
	}
	process := exec.Command(os.Args[0], "-test.run=^TestPolicyCLIHelper$")
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(name, "SLACK_") || strings.HasPrefix(name, "SLK_TEST_") || name == "XDG_CACHE_HOME" || name == "GORACE" {
			continue
		}
		process.Env = append(process.Env, variable)
	}
	process.Env = append(process.Env,
		"SLACK_CLI_READ_ONLY="+setting, "SLK_TEST_POLICY_ARGS="+string(encoded),
		"XDG_CACHE_HOME="+t.TempDir(), "GORACE=atexit_sleep_ms=0")
	output, err := process.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if failure, ok := err.(*exec.ExitError); ok {
		return failure.ExitCode(), string(output)
	}
	t.Fatal(err)
	return -1, string(output)
}

func TestReadOnlyCLIRejectsWritesBeforeNetwork(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	for command, method := range commandOperations {
		if method == "" || policy.CheckMethod(method) == nil {
			continue
		}
		t.Run(command, func(t *testing.T) {
			code, output := runPolicyCLI(t, "true", strings.Fields(command)...)
			if code != 6 || !strings.Contains(output, "read_only_violation") || !strings.Contains(output, method) || strings.Contains(output, "TEST_HTTP_REQUEST") {
				t.Fatalf("exit=%d output=%s", code, output)
			}
		})
	}
	for _, args := range [][]string{
		{"conversation", "join", "--channel", "#general"},
		{"user-groups", "members", "set", "--group", "@engineering", "--members", "@alice"},
		{"messages", "send", "--channel", "@alice", "--mrkdwn", "secret message"},
		{"api", "chat.postMessage", "--data", "@/does/not/exist"},
		{"api", "future.read", "--data", "-"},
		{"api", "assistant.search.context"},
		{"messages", "list", "--channel", "@alice", "--refresh-cache"},
		{"conversations", "info", "--channel", "@alice"},
		{"events", "stream", "--channel", "@alice"},
		{"daemon", "run", "--channel", "@alice"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, output := runPolicyCLI(t, "true", args...)
			if code != 6 || !strings.Contains(output, "read_only_violation") || strings.Contains(output, "TEST_HTTP_REQUEST") || strings.Contains(output, "secret message") {
				t.Fatalf("exit=%d output=%s", code, output)
			}
		})
	}
}

func TestReadOnlyCLIRejectsMalformedSetting(t *testing.T) {
	for _, setting := range []string{"", "tru", " false "} {
		for _, args := range [][]string{{"auth", "test"}, {"auth", "login", "--token", "xoxp-test-local"}, {"api", "users.info"}} {
			code, output := runPolicyCLI(t, setting, args...)
			if code != 2 || !strings.Contains(output, policy.EnvReadOnly) || strings.Contains(output, "TEST_HTTP_REQUEST") {
				t.Fatalf("setting=%q exit=%d output=%s", setting, code, output)
			}
		}
	}
}

func TestReadOnlyCLIAllowsReadsAndLocalChanges(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "test"}, {"auth", "login", "--token", "xoxp-test-local"},
		{"api", "users.info", "--data", `{"user":"UTEST"}`},
		{"messages", "list", "--channel", "D12345"},
		{"cache", "status"}, {"cache", "clear"}, {"daemon", "status"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, output := runPolicyCLI(t, "true", args...)
			if code != 0 || strings.Contains(output, "read_only_violation") {
				t.Fatalf("exit=%d output=%s", code, output)
			}
		})
	}
	code, output := runPolicyCLI(t, "false", "api", "chat.postMessage", "--data", `{"channel":"C123","text":"mock only"}`)
	if code != 0 || !strings.Contains(output, "TEST_HTTP_REQUEST /api/chat.postMessage") {
		t.Fatalf("default write compatibility: exit=%d output=%s", code, output)
	}
}

func TestReadOnlyCommandInventoryAndFutureCommand(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	var check func(*cobra.Command)
	check = func(command *cobra.Command) {
		if command.Run != nil || command.RunE != nil {
			key := commandOperationKey(command)
			if _, ok := commandOperations[key]; !ok && key != "api" {
				t.Errorf("runnable command needs review: %s", key)
			}
		}
		for _, child := range command.Commands() {
			check(child)
		}
	}
	check(rootCmd)
	future := &cobra.Command{Use: "future-read"}
	rootCmd.AddCommand(future)
	defer rootCmd.RemoveCommand(future)
	if err := enforceCommandPolicy(future, nil); err == nil || !strings.Contains(err.Error(), "read_only_violation") {
		t.Fatalf("unreviewed command allowed: %v", err)
	}
}

func TestReadOnlyDirectSideEffects(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	// Nil dependencies prove refusal happens before resolving a user or opening DM.
	ctx := &CommandContext{Ctx: context.Background()}
	if _, err := ctx.ResolveChannel("@alice"); err == nil || !strings.Contains(err.Error(), "conversations.open") {
		t.Fatalf("implicit DM allowed: %v", err)
	}
	if err := runAuthOAuth(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "oauth.v2.access") {
		t.Fatalf("OAuth listener allowed: %v", err)
	}
	if _, err := exchangeCodeForToken("secret-code", "client", "secret", ""); err == nil || !strings.Contains(err.Error(), "oauth.v2.access") {
		t.Fatalf("OAuth exchange allowed: %v", err)
	}
	command := &cobra.Command{}
	command.Flags().String("data", "@"+filepath.Join(t.TempDir(), "missing"), "")
	if err := runAPI(command, []string{"chat.postMessage"}); err == nil || !strings.Contains(err.Error(), "read_only_violation") {
		t.Fatalf("raw input read before policy: %v", err)
	}
}
