package slack

import (
	"net/http"
	"strings"

	slackapi "github.com/slack-go/slack"
)

// APIClient implements Client by wrapping slack-go's Client.
type APIClient struct {
	sdk *slackapi.Client
}

// New creates a new APIClient using the provided user token.
// For xoxc- tokens (client tokens), use NewWithCookie instead.
func New(userToken string) *APIClient {
	return &APIClient{sdk: slackapi.New(userToken)}
}

// NewWithCookie creates a new APIClient for xoxc- tokens that require a cookie.
// The cookie parameter should be the value of the 'd' cookie (xoxd-...).
func NewWithCookie(token, cookie string) *APIClient {
	httpClient := &http.Client{
		Transport: &cookieTransport{
			cookie: cookie,
			base:   http.DefaultTransport,
		},
	}
	return &APIClient{sdk: slackapi.New(token, slackapi.OptionHTTPClient(httpClient))}
}

// NewAuto automatically creates the appropriate client based on token type.
// If token starts with xoxc-, it requires a cookie. Otherwise, uses standard auth.
func NewAuto(token, cookie string) *APIClient {
	if strings.HasPrefix(token, "xoxc-") && cookie != "" {
		return NewWithCookie(token, cookie)
	}
	return New(token)
}

// cookieTransport is an http.RoundTripper that adds the Slack 'd' cookie to requests.
type cookieTransport struct {
	cookie string
	base   http.RoundTripper
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set("Cookie", "d="+t.cookie)
	return t.base.RoundTrip(req2)
}
