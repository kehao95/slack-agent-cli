package slack

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/policy"
	slackapi "github.com/slack-go/slack"
)

type policyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f policyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type policyDoFunc func(*http.Request) (*http.Response, error)

func (f policyDoFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func installPolicyTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func policyResponse(req *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func requirePolicyViolation(t *testing.T, err error, method string) {
	t.Helper()
	var violation *policy.ViolationError
	var coded *cerrors.ErrorWithExitCode
	if !errors.As(err, &violation) || violation.Operation != method || !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitPermission {
		t.Fatalf("expected read_only_violation for %s with exit 6, got %v", method, err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("credential leaked: %v", err)
	}
}

func TestReadOnlySDKWritesMakeZeroRequests(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	calls := 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return policyResponse(req, `{"ok":true}`), nil
	}))
	client := New("xoxp-write-capable-secret")
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(file, []byte("test image"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method string
		run    func() error
	}{
		{"chat.postMessage", func() error {
			_, err := client.PostMessage(ctx, "C123", PostMessageOptions{Text: "secret"})
			return err
		}},
		{"chat.update", func() error { _, err := client.EditMessage(ctx, "C123", "1.2", "secret"); return err }},
		{"chat.delete", func() error { _, err := client.DeleteMessage(ctx, "C123", "1.2"); return err }},
		{"conversations.open", func() error { _, err := client.OpenConversation(ctx, "", []string{"U123"}); return err }},
		{"conversations.mark", func() error { _, err := client.MarkConversationRead(ctx, "C123", "1.2"); return err }},
		{"conversations.join", func() error { _, _, _, err := client.sdk.JoinConversationContext(ctx, "C123"); return err }},
		{"reactions.add", func() error {
			return client.sdk.AddReactionContext(ctx, "eyes", slackapi.ItemRef{Channel: "C123", Timestamp: "1.2"})
		}},
		{"pins.add", func() error {
			return client.sdk.AddPinContext(ctx, "C123", slackapi.ItemRef{Channel: "C123", Timestamp: "1.2"})
		}},
		{"users.profile.set", func() error { return client.SetUserStatus(ctx, "U123", "secret", "", 0) }},
		{"usergroups.users.update", func() error { _, err := client.UpdateUserGroupMembers(ctx, "S123", []string{"U123"}); return err }},
		{"files.delete", func() error { return client.DeleteFile(ctx, "F123") }},
		{"files.sharedPublicURL", func() error { _, err := client.ShareFilePublicURL(ctx, "F123"); return err }},
		{"files.revokePublicURL", func() error { _, err := client.RevokeFilePublicURL(ctx, "F123"); return err }},
		{"files.getUploadURLExternal", func() error { _, err := client.UploadLocalFile(ctx, file, UploadFileOptions{}); return err }},
		{"files.getUploadURLExternal", func() error { _, err := client.UploadImage(ctx, "C123", file, UploadImageOptions{}); return err }},
	} {
		t.Run(test.method, func(t *testing.T) { requirePolicyViolation(t, test.run(), test.method) })
	}
	if calls != 0 {
		t.Fatalf("blocked writes made %d requests", calls)
	}
}

func TestReadOnlyRawAndBespokePaths(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	calls := 0
	transport := policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return policyResponse(req, `{"ok":true,"items":[],"file":{"id":"F123"},"channels":[]}`), nil
	})
	// Intentionally bypass constructors: entry checks and rawRequest must still guard.
	client := &APIClient{token: "xoxp-write-capable-secret", endpoint: "https://slack.com/api/", rawHTTPClient: &http.Client{Transport: transport}}
	ctx := context.Background()
	for _, method := range []string{"chat.postMessage", "files.completeUploadExternal", "future.list", "assistant.search.context"} {
		_, err := client.CallAPI(ctx, method, map[string]interface{}{"text": "secret"}, CallAPIOptions{})
		requirePolicyViolation(t, err, method)
	}
	_, err := client.postSlackListsMethod(ctx, "slackLists.items.create", nil)
	requirePolicyViolation(t, err, "slackLists.items.create")
	_, err = client.postSlackMethodForm(ctx, "files.delete", nil)
	requirePolicyViolation(t, err, "files.delete")
	if calls != 0 {
		t.Fatalf("blocked raw calls made %d requests", calls)
	}
	if _, err := client.CallAPI(ctx, "conversations.list", nil, CallAPIOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListSlackListItems(ctx, ListItemsParams{ListID: "F123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSlackList(ctx, "F123"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected three reads, got %d", calls)
	}
}

func TestReadOnlyConstructorOptionsCannotBypassPolicy(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	customCalls, guardedCalls := 0, 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		guardedCalls++
		if req.URL.Host != "unit.invalid" || req.URL.Path != "/test/auth.test" {
			t.Errorf("custom API endpoint not preserved: %s", req.URL)
		}
		return policyResponse(req, `{"ok":true,"user_id":"U123","team_id":"T123"}`), nil
	}))
	custom := policyDoFunc(func(req *http.Request) (*http.Response, error) {
		customCalls++
		return policyResponse(req, `{"ok":true}`), nil
	})
	client := New("secret", slackapi.OptionAPIURL("https://unit.invalid/test/"), slackapi.OptionHTTPClient(custom))
	_, err := client.PostMessage(context.Background(), "C123", PostMessageOptions{Text: "secret"})
	requirePolicyViolation(t, err, "chat.postMessage")
	if _, err := client.AuthTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if customCalls != 0 || guardedCalls != 1 {
		t.Fatalf("custom=%d guarded=%d", customCalls, guardedCalls)
	}
	// Existing injection behavior stays intact outside read-only mode.
	t.Setenv(policy.EnvReadOnly, "false")
	client = New("secret", slackapi.OptionHTTPClient(custom))
	if _, err := client.AuthTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if customCalls != 1 {
		t.Fatalf("normal custom HTTP option was ignored: %d", customCalls)
	}
}

func TestReadOnlyCookieAndSocketClients(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	calls := 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Cookie") != "d=xoxd-secret" {
			t.Error("cookie missing on allowed read")
		}
		return policyResponse(req, `{"ok":true,"user_id":"U123","team_id":"T123","url":"wss://unit.invalid/socket"}`), nil
	}))
	client := NewAuto("xoxc-secret", "xoxd-secret")
	_, err := client.PostMessage(context.Background(), "C123", PostMessageOptions{Text: "secret"})
	requirePolicyViolation(t, err, "chat.postMessage")
	if _, err := client.AuthTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	socket := NewSocketModeClient("xoxc-secret", "xoxd-secret", "xapp-secret")
	_, _, err = socket.PostMessageContext(context.Background(), "C123", slackapi.MsgOptionText("secret", false))
	requirePolicyViolation(t, err, "chat.postMessage")
	if _, _, err := socket.StartSocketModeContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected auth and connection read calls only: %d", calls)
	}
}

func TestReadOnlyDownloadCapabilityAndRedirects(t *testing.T) {
	const download = "https://files.slack.com/files-pri/T123-F123/report.pdf?token=secret"
	for _, test := range []struct {
		name, target string
		allowed      bool
	}{
		{"direct", "", true},
		{"same-origin-content", "https://files.slack.com/files-pri/T123-F123/new.pdf", true},
		{"Slack-write", "https://slack.com/api/chat.postMessage?text=secret", false},
		{"unknown-API", "https://slack.com/api/future.list?token=secret", false},
		{"file-host-API", "https://files.slack.com/api/chat.postMessage", false},
		{"cross-origin", "https://cdn.invalid/report.pdf", false},
		{"encoded-path", "https://files.slack.com/files-pri/%2e%2e/api/chat.postMessage", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(policy.EnvReadOnly, "true")
			calls := 0
			installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				response := policyResponse(req, "file bytes")
				if calls == 1 && test.target != "" {
					response.StatusCode = http.StatusFound
					response.Header.Set("Location", test.target)
				}
				return response, nil
			}))
			var content strings.Builder
			err := New("secret").DownloadFile(context.Background(), download, &content)
			if test.allowed {
				if err != nil || content.String() != "file bytes" {
					t.Fatalf("download: content=%q err=%v", content.String(), err)
				}
			} else {
				requirePolicyViolation(t, err, "files.download")
				if calls != 1 {
					t.Fatalf("redirect reached blocked destination: %d requests", calls)
				}
			}
		})
	}
}

func TestReadOnlyDownloadIsNotAGenericGETExemption(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	calls := 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return policyResponse(req, ""), nil }))
	client := New("secret")
	for _, target := range []string{"https://slack.com/api/chat.postMessage?token=secret", "https://slack.com/api/future.list", "https://files.slack.com/files-pri/../api/chat.postMessage", "http://files.slack.com/files-pri/T-F/file"} {
		err := client.DownloadFile(context.Background(), target, io.Discard)
		requirePolicyViolation(t, err, "files.download")
	}
	// Calling the SDK download method directly does not grant the capability.
	err := client.sdk.GetFileContext(context.Background(), "https://files.slack.com/files-pri/T-F/file", io.Discard)
	requirePolicyViolation(t, err, "unrecognized HTTP request")
	// Even a scoped context cannot authorize an upload request.
	req, _ := http.NewRequestWithContext(withDownloadPermission(context.Background()), http.MethodPost, "https://files.slack.com/files-pri/T-F/file", nil)
	_, err = rawRequest(client.rawHTTPClient, req)
	requirePolicyViolation(t, err, "files.download")
	if calls != 0 {
		t.Fatalf("blocked file requests reached transport: %d", calls)
	}
}

func TestReadOnlyRawRedirectCannotReachWrite(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	calls := 0
	client := &APIClient{endpoint: "https://slack.com/api/", rawHTTPClient: &http.Client{Transport: policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		response := policyResponse(req, "")
		response.StatusCode = http.StatusTemporaryRedirect
		response.Header.Set("Location", "https://slack.com/api/chat.postMessage?token=secret")
		return response, nil
	})}}
	_, err := client.CallAPI(context.Background(), "users.info", map[string]interface{}{"user": "U123"}, CallAPIOptions{})
	requirePolicyViolation(t, err, "chat.postMessage")
	if calls != 1 {
		t.Fatalf("write redirect made a request: %d", calls)
	}
}

func TestReadOnlyMalformedEnvironmentBlocksAllClientPaths(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "")
	calls := 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return policyResponse(req, ""), nil }))
	client := New("secret", slackapi.OptionHTTPClient(policyDoFunc(func(req *http.Request) (*http.Response, error) { calls++; return policyResponse(req, ""), nil })))
	for _, run := range []func() error{
		func() error { _, err := client.AuthTest(context.Background()); return err },
		func() error {
			_, err := client.CallAPI(context.Background(), "users.info", nil, CallAPIOptions{})
			return err
		},
		func() error {
			_, err := client.postSlackMethodForm(context.Background(), "files.info", url.Values{})
			return err
		},
	} {
		var coded *cerrors.ErrorWithExitCode
		if err := run(); !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitConfig {
			t.Fatalf("expected config error: %v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("malformed setting permitted %d requests", calls)
	}
}
