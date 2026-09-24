package slack

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/policy"
)

type downloadContextKey struct{}

// Only DownloadFile can grant this capability. API calls never receive it.
func withDownloadPermission(ctx context.Context) context.Context {
	return context.WithValue(ctx, downloadContextKey{}, true)
}

func isPrivateFileURL(u *url.URL) bool {
	return u != nil && u.Scheme == "https" && u.Host == "files.slack.com" &&
		u.User == nil && u.Fragment == "" && u.RawPath == "" &&
		strings.HasPrefix(u.Path, "/files-pri/") && path.Clean(u.Path) == u.Path
}

type policyTransport struct{ base http.RoundTripper }

func (t *policyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	enabled, err := policy.ReadOnly()
	if err != nil {
		return nil, err
	}
	if enabled {
		if allowed, _ := req.Context().Value(downloadContextKey{}).(bool); allowed {
			// Recheck every redirect: no API endpoint, cross-origin URL, path
			// escape, or non-GET request can inherit download permission.
			if req.Method != http.MethodGet || !isPrivateFileURL(req.URL) {
				return nil, policy.Deny("files.download")
			}
		} else {
			method := path.Base(req.URL.Path)
			if req.URL.RawPath != "" || path.Clean(req.URL.Path) != req.URL.Path || ValidateAPIMethod(method) != nil {
				return nil, policy.Deny("unrecognized HTTP request")
			}
			if err := policy.CheckMethod(method); err != nil {
				return nil, err
			}
		}
	}
	return t.base.RoundTrip(req)
}

// policyHTTPClient removes net/http's URL wrapper from policy errors so query
// parameters and private download URLs cannot leak into diagnostics.
type policyHTTPClient struct{ client *http.Client }

func (c policyHTTPClient) Do(req *http.Request) (*http.Response, error) {
	response, err := c.client.Do(req)
	var coded *cerrors.ErrorWithExitCode
	if errors.As(err, &coded) {
		return response, coded
	}
	return response, err
}

func guardedHTTPClient(cookie string) *http.Client {
	var base http.RoundTripper = &formAuthTransport{base: http.DefaultTransport}
	if cookie != "" {
		base = &cookieTransport{cookie: cookie, base: base}
	}
	return &http.Client{Transport: &policyTransport{base: base}}
}

// rawRequest also guards manually constructed clients used by internal callers
// and tests. Cloning preserves timeout/redirect behavior without mutating them.
func rawRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	copy := *client
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = &policyTransport{base: base}
	return (policyHTTPClient{client: &copy}).Do(req)
}
