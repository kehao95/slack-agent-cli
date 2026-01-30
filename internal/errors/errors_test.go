package errors

import (
	"errors"
	"fmt"
	"testing"

	slackapi "github.com/slack-go/slack"
)

func TestClassifySlackError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ExitSuccess,
		},
		{
			name:     "rate limit error string",
			err:      fmt.Errorf("rate_limit exceeded"),
			expected: ExitRateLimit,
		},
		{
			name:     "rate limit error natural language",
			err:      fmt.Errorf("Slack API rate limit exceeded"),
			expected: ExitRateLimit,
		},
		{
			name:     "invalid auth",
			err:      fmt.Errorf("invalid_auth"),
			expected: ExitAuth,
		},
		{
			name:     "token revoked",
			err:      fmt.Errorf("token_revoked"),
			expected: ExitAuth,
		},
		{
			name:     "not authed",
			err:      fmt.Errorf("not_authed"),
			expected: ExitAuth,
		},
		{
			name:     "missing scope",
			err:      fmt.Errorf("missing_scope: channels:read"),
			expected: ExitPermission,
		},
		{
			name:     "not allowed",
			err:      fmt.Errorf("not_allowed"),
			expected: ExitPermission,
		},
		{
			name:     "channel not found",
			err:      fmt.Errorf("channel_not_found"),
			expected: ExitNotFound,
		},
		{
			name:     "user not found",
			err:      fmt.Errorf("user_not_found"),
			expected: ExitNotFound,
		},
		{
			name:     "message not found",
			err:      fmt.Errorf("message_not_found"),
			expected: ExitNotFound,
		},
		{
			name:     "not in channel",
			err:      fmt.Errorf("not_in_channel"),
			expected: ExitNotFound,
		},
		{
			name:     "network timeout",
			err:      fmt.Errorf("context deadline exceeded: timeout"),
			expected: ExitNetwork,
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("dial tcp: connection refused"),
			expected: ExitNetwork,
		},
		{
			name:     "general error",
			err:      fmt.Errorf("some other error"),
			expected: ExitGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifySlackError(tt.err)
			if result != tt.expected {
				t.Errorf("ClassifySlackError() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestClassifySlackError_RateLimitType(t *testing.T) {
	// Test with actual RateLimitedError type
	rlErr := &slackapi.RateLimitedError{
		RetryAfter: 30,
	}

	result := ClassifySlackError(rlErr)
	if result != ExitRateLimit {
		t.Errorf("ClassifySlackError(RateLimitedError) = %d, want %d", result, ExitRateLimit)
	}
}

func TestNewErrorWithCode(t *testing.T) {
	err := NewErrorWithCode(ExitConfig, "config file not found: %s", "/path/to/config")

	if err.ExitCode != ExitConfig {
		t.Errorf("ExitCode = %d, want %d", err.ExitCode, ExitConfig)
	}

	expectedMsg := "config file not found: /path/to/config"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestWrapWithCode(t *testing.T) {
	innerErr := fmt.Errorf("connection refused")
	wrappedErr := WrapWithCode(ExitNetwork, innerErr, "failed to connect to Slack")

	if wrappedErr.ExitCode != ExitNetwork {
		t.Errorf("ExitCode = %d, want %d", wrappedErr.ExitCode, ExitNetwork)
	}

	// Check that the error wraps the original
	if !errors.Is(wrappedErr, innerErr) {
		t.Error("WrapWithCode should wrap the original error")
	}
}

func TestErrorWithExitCode_Unwrap(t *testing.T) {
	innerErr := fmt.Errorf("inner error")
	wrappedErr := &ErrorWithExitCode{
		Err:      innerErr,
		ExitCode: ExitGeneral,
	}

	unwrapped := wrappedErr.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}
}

func TestMissingScopeError(t *testing.T) {
	err := MissingScopeError("list channels", "channels:read", "groups:read")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("MissingScopeError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitPermission {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitPermission)
	}

	errMsg := err.Error()
	if !containsAll(errMsg, "list channels", "channels:read", "groups:read") {
		t.Errorf("Error message missing expected content: %q", errMsg)
	}
}

func TestChannelNotFoundError(t *testing.T) {
	err := ChannelNotFoundError("#general")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("ChannelNotFoundError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitNotFound {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitNotFound)
	}

	errMsg := err.Error()
	if !containsAll(errMsg, "#general", "cache populate channels") {
		t.Errorf("Error message missing expected hint: %q", errMsg)
	}
}

func TestUserNotFoundError(t *testing.T) {
	err := UserNotFoundError("@alice")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("UserNotFoundError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitNotFound {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitNotFound)
	}

	errMsg := err.Error()
	if !containsAll(errMsg, "@alice", "cache populate users") {
		t.Errorf("Error message missing expected hint: %q", errMsg)
	}
}

func TestConfigError(t *testing.T) {
	err := ConfigError("invalid token format")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("ConfigError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitConfig {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitConfig)
	}
}

func TestAuthError(t *testing.T) {
	err := AuthError("token expired")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("AuthError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitAuth {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitAuth)
	}
}

func TestNetworkError(t *testing.T) {
	err := NetworkError("connection timeout")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("NetworkError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitNetwork {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitNetwork)
	}
}

func TestRateLimitError(t *testing.T) {
	err := RateLimitError("30 seconds")

	var errWithCode *ErrorWithExitCode
	if !errors.As(err, &errWithCode) {
		t.Fatal("RateLimitError should return ErrorWithExitCode")
	}

	if errWithCode.ExitCode != ExitRateLimit {
		t.Errorf("ExitCode = %d, want %d", errWithCode.ExitCode, ExitRateLimit)
	}

	errMsg := err.Error()
	if !containsAll(errMsg, "rate limit", "30 seconds") {
		t.Errorf("Error message missing expected content: %q", errMsg)
	}
}

func TestIsMissingScopeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "missing_scope error",
			err:      fmt.Errorf("missing_scope: channels:read"),
			expected: true,
		},
		{
			name:     "not_allowed error",
			err:      fmt.Errorf("not_allowed"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMissingScopeError(tt.err)
			if result != tt.expected {
				t.Errorf("IsMissingScopeError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "channel not found",
			err:      ChannelNotFoundError("#test"),
			expected: true,
		},
		{
			name:     "user not found",
			err:      UserNotFoundError("@test"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some error"),
			expected: false,
		},
		{
			name:     "wrapped not found",
			err:      fmt.Errorf("wrapped: %w", ChannelNotFoundError("#test")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFoundError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to check if a string contains all substrings
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
