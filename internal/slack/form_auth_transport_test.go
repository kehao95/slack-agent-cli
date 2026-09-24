package slack

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/policy"
)

func TestSDKAuthTestUsesHeaderAuthentication(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "true")
	for _, token := range []string{"xoxp-placeholder", "xoxb-placeholder", "xoxc-placeholder"} {
		t.Run(token[:4], func(t *testing.T) {
			calls := 0
			installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.String() != "https://slack.com/api/auth.test" || req.Header.Get("Authorization") != "Bearer "+token {
					t.Fatal("SDK auth.test did not use header authentication")
				}
				if err := req.ParseForm(); err != nil || req.PostForm.Has("token") {
					t.Fatal("SDK auth.test still included a form token")
				}
				if token[:4] == "xoxc" && req.Header.Get("Cookie") != "d=xoxd-cookie" {
					t.Fatal("cookie authentication was lost")
				}
				return policyResponse(req, `{"ok":true,"user_id":"U123","team_id":"T123"}`), nil
			}))
			if _, err := NewAuto(token, "xoxd-cookie").AuthTest(context.Background()); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("expected one request, got %d", calls)
			}
		})
	}
}

func TestFormAuthenticationPreservesPayloadAndReplay(t *testing.T) {
	values := url.Values{"token": {"xoxp-placeholder"}, "text": {"snowman ☃ & plus+"}, "user": {"U1", "U2"}, "empty": {""}}
	original := values.Encode()
	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("Content-Length", "999")
	headers := req.Header.Clone()
	delete(values, "token")
	calls := 0
	transport := &formAuthTransport{base: policyRoundTripFunc(func(outgoing *http.Request) (*http.Response, error) {
		calls++
		if outgoing == req || outgoing.Header.Get("Authorization") != "Bearer xoxp-placeholder" {
			t.Fatal("request was not cloned with header authentication")
		}
		body, _ := io.ReadAll(outgoing.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil || !reflect.DeepEqual(form, values) {
			t.Fatalf("business fields changed: %v", form)
		}
		if outgoing.ContentLength != int64(len(body)) || outgoing.Header.Get("Content-Length") != "" {
			t.Fatal("replacement body length was not updated")
		}
		replay, err := outgoing.GetBody()
		if err != nil {
			t.Fatal(err)
		}
		defer replay.Close()
		replayed, _ := io.ReadAll(replay)
		if string(replayed) != string(body) {
			t.Fatal("request replay changed the normalized payload")
		}
		return policyResponse(outgoing, `{"ok":true}`), nil
	})}
	for range 2 {
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if calls != 2 || !reflect.DeepEqual(req.Header, headers) || req.ContentLength != int64(len(original)) {
		t.Fatal("caller request metadata changed")
	}
	body, _ := io.ReadAll(req.Body)
	replay, _ := req.GetBody()
	defer replay.Close()
	replayed, _ := io.ReadAll(replay)
	if string(body) != original || string(replayed) != original {
		t.Fatal("caller body or replay changed")
	}
}

func TestFormAuthenticationDoesNotRewriteOtherRequests(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, contentType, payload string
	}{
		{"upload", "https://files.slack.com/upload/v1/abc", "application/x-www-form-urlencoded", "token=business-token"},
		{"foreign", "https://example.com/api/auth.test", "application/x-www-form-urlencoded", "token=business-token"},
		{"plaintext", "http://slack.com/api/auth.test", "application/x-www-form-urlencoded", "token=business-token"},
		{"path escape", "https://slack.com/api/../upload", "application/x-www-form-urlencoded", "token=business-token"},
		{"JSON", "https://slack.com/api/auth.test", "application/json", `{"token":"business-token"}`},
		{"multipart", "https://slack.com/api/files.upload", "multipart/form-data; boundary=abc", "token=business-token"},
		{"no token", "https://slack.com/api/auth.test", "application/x-www-form-urlencoded", "team_id=T123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, tc.endpoint, strings.NewReader(tc.payload))
			req.Header.Set("Content-Type", tc.contentType)
			outgoing, err := headerAuthRequest(req)
			if err != nil || outgoing != req {
				t.Fatalf("unrelated request changed: %v", err)
			}
		})
	}
}

func TestFormAuthenticationPreservesAlreadyAuthenticatedRequest(t *testing.T) {
	const payload = "token=business+token&field=unsorted"
	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/oauth.v2.exchange", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer existing")
	headers := req.Header.Clone()
	req.GetBody = func() (io.ReadCloser, error) {
		t.Fatal("already authenticated request body must not be inspected")
		return nil, nil
	}
	outgoing, err := headerAuthRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	defer outgoing.Body.Close()
	if outgoing != req || !reflect.DeepEqual(req.Header, headers) || req.ContentLength != int64(len(payload)) {
		t.Fatal("already authenticated request changed")
	}
	body, _ := io.ReadAll(outgoing.Body)
	if string(body) != payload {
		t.Fatal("business payload changed")
	}
}

func TestCallAPIPreservesBusinessTokenWithDefaultHTTPClient(t *testing.T) {
	t.Setenv(policy.EnvReadOnly, "false")
	calls := 0
	installPolicyTransport(t, policyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != "https://slack.com/api/oauth.v2.exchange" || req.Header.Get("Authorization") != "Bearer xoxp-auth" {
			t.Fatal("raw API endpoint or authentication changed")
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(req.PostForm, url.Values{"token": {"business-token"}, "client_id": {"123"}}) {
			t.Fatalf("raw API payload changed: %v", req.PostForm)
		}
		return policyResponse(req, `{"ok":true}`), nil
	}))
	_, err := New("xoxp-auth").CallAPI(context.Background(), "oauth.v2.exchange", map[string]interface{}{
		"token": "business-token", "client_id": "123",
	}, CallAPIOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

func TestFormAuthenticationRejectsAmbiguousOrMalformedForms(t *testing.T) {
	for _, body := range []string{"token=secret&token=other", "token=secret%XX"} {
		req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/auth.test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_, err := headerAuthRequest(req)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected credential-free validation error, got %v", err)
		}
	}
}
