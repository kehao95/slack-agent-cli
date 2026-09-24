package slack

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// formAuthTransport gives SDK form calls the same header authentication as its
// JSON calls. This lets credential brokers replace credentials at the HTTP
// boundary without parsing or changing business payloads.
type formAuthTransport struct{ base http.RoundTripper }

func (t *formAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	outgoing, err := headerAuthRequest(req)
	if (err != nil || outgoing != req) && req.Body != nil {
		// The next transport receives a replacement body; we own the original.
		defer req.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return t.base.RoundTrip(outgoing)
}

func headerAuthRequest(req *http.Request) (*http.Request, error) {
	// Authenticated raw API requests can have a business field named token
	// (for example oauth.v2.exchange); their payload must remain untouched.
	if req.Header.Get("Authorization") != "" {
		return req, nil
	}
	u := req.URL
	if req.Method != http.MethodPost || u == nil || u.Scheme != "https" || u.Host != "slack.com" ||
		u.User != nil || u.RawPath != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/api/") ||
		ValidateAPIMethod(strings.TrimPrefix(u.Path, "/api/")) != nil {
		return req, nil
	}
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	// slack-go supplies GetBody for form requests. Leave streaming uploads and
	// custom request bodies untouched rather than consuming the caller's stream.
	if err != nil || mediaType != "application/x-www-form-urlencoded" || req.GetBody == nil {
		return req, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, errors.New("cannot replay Slack authentication form")
	}
	encoded, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return nil, errors.New("cannot read Slack authentication form")
	}
	form, err := url.ParseQuery(string(encoded))
	if err != nil {
		return nil, errors.New("invalid Slack authentication form")
	}
	tokens := form["token"]
	if len(tokens) == 0 || (len(tokens) == 1 && tokens[0] == "") {
		return req, nil
	}
	if len(tokens) != 1 {
		return nil, errors.New("ambiguous Slack authentication form")
	}
	form.Del("token")
	payload := form.Encode()
	copy := req.Clone(req.Context())
	copy.Header.Set("Authorization", "Bearer "+tokens[0])
	copy.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(payload)), nil
	}
	copy.Body, _ = copy.GetBody()
	copy.ContentLength = int64(len(payload))
	copy.Header.Del("Content-Length")
	return copy, nil
}
