package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
)

var apiMethodPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)+$`)

// ValidateAPIMethod prevents raw API method names from escaping the configured
// Slack API endpoint and enforces Slack's family.method naming convention.
func ValidateAPIMethod(method string) error {
	if !apiMethodPattern.MatchString(method) {
		return fmt.Errorf("invalid Slack API method %q: expected family.method", method)
	}
	return nil
}

// CallAPIOptions controls retry behavior for raw Slack Web API calls.
type CallAPIOptions struct {
	// MaxRetries is the number of additional attempts after a 429 response.
	MaxRetries int
}

// APIError is returned when Slack responds with ok=false.
type APIError struct {
	Method   string
	Code     string
	Needed   string
	Provided string
}

func (e *APIError) Error() string {
	if e.Needed != "" || e.Provided != "" {
		return fmt.Sprintf("%s: %s (needed: %s provided: %s)", e.Method, e.Code, e.Needed, e.Provided)
	}
	return fmt.Sprintf("%s: %s", e.Method, e.Code)
}

// CallAPI invokes a Slack Web API method using a JSON request body. It preserves
// the complete JSON response so callers can use methods not yet wrapped by the SDK.
func (c *APIClient) CallAPI(ctx context.Context, method string, payload map[string]interface{}, opts CallAPIOptions) (json.RawMessage, error) {
	if c == nil || c.rawHTTPClient == nil {
		return nil, fmt.Errorf("Slack API client is not initialized")
	}
	if err := ValidateAPIMethod(method); err != nil {
		return nil, err
	}
	if opts.MaxRetries < 0 {
		return nil, fmt.Errorf("max retries cannot be negative")
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	body, err := encodeAPIForm(payload)
	if err != nil {
		return nil, fmt.Errorf("encode %s request: %w", method, err)
	}

	for attempt := 0; ; attempt++ {
		respBody, retryAfter, err := c.callAPIAttempt(ctx, method, body)
		if err == nil {
			return respBody, nil
		}

		var rateLimitErr *slackapi.RateLimitedError
		if !errors.As(err, &rateLimitErr) || attempt >= opts.MaxRetries {
			return nil, err
		}
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		if err := c.waitForRetry(ctx, retryAfter); err != nil {
			return nil, err
		}
	}
}

// encodeAPIForm keeps the CLI's JSON-object input contract while using Slack's
// universally supported form transport. Slack endpoints in some workspaces and
// token modes ignore application/json bodies and report required fields as
// missing. Structured values remain JSON strings, matching Slack's form API
// convention for blocks, attachments, and other composite parameters.
func encodeAPIForm(payload map[string]interface{}) ([]byte, error) {
	values := url.Values{}
	for key, value := range payload {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("request field name cannot be empty")
		}
		if text, ok := value.(string); ok {
			values.Set(key, text)
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", key, err)
		}
		values.Set(key, string(encoded))
	}
	return []byte(values.Encode()), nil
}

func (c *APIClient) waitForRetry(ctx context.Context, delay time.Duration) error {
	if c.retryWait != nil {
		return c.retryWait(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *APIClient) callAPIAttempt(ctx context.Context, method string, body []byte) (json.RawMessage, time.Duration, error) {
	endpoint := strings.TrimRight(c.endpoint, "/") + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(c.cookie) != "" {
		req.Header.Set("Cookie", "d="+c.cookie)
	}

	resp, err := c.rawHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s response: %w", method, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, retryAfter, &slackapi.RateLimitedError{RetryAfter: retryAfter}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, 0, fmt.Errorf("%s: Slack API returned HTTP %d", method, resp.StatusCode)
	}
	if !json.Valid(respBody) {
		return nil, 0, fmt.Errorf("decode %s response: invalid JSON", method)
	}

	var envelope struct {
		OK       *bool  `json:"ok"`
		Error    string `json:"error"`
		Needed   string `json:"needed"`
		Provided string `json:"provided"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, 0, fmt.Errorf("decode %s response: %w", method, err)
	}
	if envelope.OK != nil && !*envelope.OK {
		return nil, 0, &APIError{Method: method, Code: envelope.Error, Needed: envelope.Needed, Provided: envelope.Provided}
	}
	return json.RawMessage(respBody), 0, nil
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// NextCursor extracts Slack's standard cursor-pagination metadata.
func NextCursor(response json.RawMessage) string {
	var envelope struct {
		ResponseMetadata struct {
			NextCursor string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
	if json.Unmarshal(response, &envelope) != nil {
		return ""
	}
	return strings.TrimSpace(envelope.ResponseMetadata.NextCursor)
}
