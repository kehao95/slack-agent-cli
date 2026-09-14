package slack

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/policy"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// APIClient implements Client by wrapping slack-go's Client.
type APIClient struct {
	sdk           *slackapi.Client
	token         string
	cookie        string
	endpoint      string
	rawHTTPClient *http.Client
	retryWait     func(context.Context, time.Duration) error
}

// New creates a new APIClient using the provided user token.
// For xoxc- tokens (client tokens), use NewWithCookie instead.
func New(userToken string, options ...slackapi.Option) *APIClient {
	httpClient := guardedHTTPClient("")
	sdkOptions := append([]slackapi.Option{slackapi.OptionHTTPClient(policyHTTPClient{client: httpClient})}, options...)
	// SDK HTTP options are opaque and cannot be wrapped through public APIs.
	// In restricted mode our final option must win; normal mode retains the
	// original custom-client behavior. Endpoint and other options still apply.
	if readOnly, err := policy.ReadOnly(); readOnly || err != nil {
		sdkOptions = append(sdkOptions, slackapi.OptionHTTPClient(policyHTTPClient{client: httpClient}))
	}
	return &APIClient{
		sdk:           slackapi.New(userToken, sdkOptions...),
		token:         userToken,
		endpoint:      slackapi.APIURL,
		rawHTTPClient: httpClient,
	}
}

// NewWithCookie creates a new APIClient for xoxc- tokens that require a cookie.
// The cookie parameter should be the value of the 'd' cookie (xoxd-...).
func NewWithCookie(token, cookie string) *APIClient {
	httpClient := guardedHTTPClient(cookie)
	return &APIClient{
		sdk:           slackapi.New(token, slackapi.OptionHTTPClient(policyHTTPClient{client: httpClient})),
		token:         token,
		cookie:        cookie,
		endpoint:      slackapi.APIURL,
		rawHTTPClient: httpClient,
	}
}

// NewAuto automatically creates the appropriate client based on token type.
// If token starts with xoxc-, it requires a cookie. Otherwise, uses standard auth.
func NewAuto(token, cookie string) *APIClient {
	if strings.HasPrefix(token, "xoxc-") && cookie != "" {
		return NewWithCookie(token, cookie)
	}
	return New(token)
}

// NewSocketModeClient creates a socketmode client using the existing user token model plus an
// app-level token for Socket Mode connection management.
func NewSocketModeClient(token, cookie, appToken string) *socketmode.Client {
	if !strings.HasPrefix(token, "xoxc-") {
		cookie = ""
	}
	api := slackapi.New(token,
		slackapi.OptionHTTPClient(policyHTTPClient{client: guardedHTTPClient(cookie)}),
		slackapi.OptionAppLevelToken(appToken),
	)
	return socketmode.New(api)
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
