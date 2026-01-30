package errors

import (
	"errors"
	"fmt"
	"os"
	"strings"

	slackapi "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// Exit codes as defined in docs/DESIGN.md section 8.1
const (
	ExitSuccess    = 0 // Success
	ExitGeneral    = 1 // General error
	ExitConfig     = 2 // Configuration error (missing config, invalid tokens)
	ExitAuth       = 3 // Authentication error (invalid/expired tokens)
	ExitRateLimit  = 4 // Rate limit exceeded
	ExitNetwork    = 5 // Network error
	ExitPermission = 6 // Permission denied (missing scopes)
	ExitNotFound   = 7 // Resource not found (channel, user, message)
)

// ErrorWithExitCode wraps an error with a specific exit code.
type ErrorWithExitCode struct {
	Err      error
	ExitCode int
}

func (e *ErrorWithExitCode) Error() string {
	return e.Err.Error()
}

func (e *ErrorWithExitCode) Unwrap() error {
	return e.Err
}

// NewErrorWithCode creates an error with a specific exit code.
func NewErrorWithCode(code int, format string, args ...interface{}) *ErrorWithExitCode {
	return &ErrorWithExitCode{
		Err:      fmt.Errorf(format, args...),
		ExitCode: code,
	}
}

// WrapWithCode wraps an existing error with an exit code.
func WrapWithCode(code int, err error, format string, args ...interface{}) *ErrorWithExitCode {
	return &ErrorWithExitCode{
		Err:      fmt.Errorf(format+": %w", append(args, err)...),
		ExitCode: code,
	}
}

// ClassifySlackError examines a Slack API error and returns an appropriate exit code.
func ClassifySlackError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	errStr := err.Error()

	// Check for rate limit error (type assertion)
	var rateLimitErr *slackapi.RateLimitedError
	if errors.As(err, &rateLimitErr) {
		return ExitRateLimit
	}

	// Check for rate limit in error string
	if strings.Contains(errStr, "rate_limit") || strings.Contains(errStr, "rate limit") {
		return ExitRateLimit
	}

	// Check for authentication errors
	if strings.Contains(errStr, "invalid_auth") ||
		strings.Contains(errStr, "token_revoked") ||
		strings.Contains(errStr, "account_inactive") ||
		strings.Contains(errStr, "not_authed") {
		return ExitAuth
	}

	// Check for permission/scope errors
	if strings.Contains(errStr, "missing_scope") ||
		strings.Contains(errStr, "not_allowed") ||
		strings.Contains(errStr, "cannot_dm_bot") ||
		strings.Contains(errStr, "access_denied") {
		return ExitPermission
	}

	// Check for not found errors
	if strings.Contains(errStr, "not_found") ||
		strings.Contains(errStr, "channel_not_found") ||
		strings.Contains(errStr, "user_not_found") ||
		strings.Contains(errStr, "message_not_found") ||
		strings.Contains(errStr, "not_in_channel") {
		return ExitNotFound
	}

	// Check for network errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "no such host") {
		return ExitNetwork
	}

	// Default to general error
	return ExitGeneral
}

// HandleCommandError processes an error from a command and sets the appropriate exit code.
// It should be called in the RunE function of cobra commands.
func HandleCommandError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	// Check if error already has an exit code
	var errWithCode *ErrorWithExitCode
	if errors.As(err, &errWithCode) {
		cmd.SilenceUsage = true
		return errWithCode
	}

	// Classify Slack API errors
	exitCode := ClassifySlackError(err)

	// Special handling for config errors
	if strings.Contains(err.Error(), "load config") ||
		strings.Contains(err.Error(), "invalid config") ||
		strings.Contains(err.Error(), "config file") {
		exitCode = ExitConfig
	}

	// Wrap with appropriate exit code
	cmd.SilenceUsage = true
	return &ErrorWithExitCode{
		Err:      err,
		ExitCode: exitCode,
	}
}

// Execute runs a cobra command and exits with the appropriate code.
// This should be used in main.go to ensure proper exit codes.
func Execute(rootCmd *cobra.Command) {
	err := rootCmd.Execute()
	if err != nil {
		var errWithCode *ErrorWithExitCode
		if errors.As(err, &errWithCode) {
			os.Exit(errWithCode.ExitCode)
		}
		os.Exit(ExitGeneral)
	}
	os.Exit(ExitSuccess)
}

// Scope error helpers

// MissingScopeError creates a user-friendly error for missing OAuth scopes.
func MissingScopeError(operation string, requiredScopes ...string) error {
	scopeList := strings.Join(requiredScopes, ", ")
	return NewErrorWithCode(
		ExitPermission,
		"missing OAuth scope for %s\nRequired scope(s): %s\n\nTo fix:\n  1. Add the scope(s) to your Slack app manifest\n  2. Reinstall the app to your workspace\n  3. Update your config with the new token",
		operation,
		scopeList,
	)
}

// NotFoundError creates a user-friendly error for missing resources.
func NotFoundError(resourceType, identifier string, hint string) error {
	msg := fmt.Sprintf("%s not found: %s", resourceType, identifier)
	if hint != "" {
		msg += "\n" + hint
	}
	return NewErrorWithCode(ExitNotFound, "%s", msg)
}

// ChannelNotFoundError creates a specific error for missing channels with helpful hints.
func ChannelNotFoundError(channel string) error {
	hint := "Hint: Run 'slack-agent-cli cache populate channels --all' to refresh the channel cache"
	return NotFoundError("channel", channel, hint)
}

// UserNotFoundError creates a specific error for missing users with helpful hints.
func UserNotFoundError(user string) error {
	hint := "Hint: Run 'slack-agent-cli cache populate users --all' to refresh the user cache"
	return NotFoundError("user", user, hint)
}

// ConfigError creates a configuration-related error.
func ConfigError(msg string, args ...interface{}) error {
	return NewErrorWithCode(ExitConfig, msg, args...)
}

// AuthError creates an authentication-related error.
func AuthError(msg string, args ...interface{}) error {
	return NewErrorWithCode(ExitAuth, msg, args...)
}

// NetworkError creates a network-related error.
func NetworkError(msg string, args ...interface{}) error {
	return NewErrorWithCode(ExitNetwork, msg, args...)
}

// RateLimitError creates a rate limit error with retry information.
func RateLimitError(retryAfter string) error {
	return NewErrorWithCode(
		ExitRateLimit,
		"Slack API rate limit exceeded\nRetry after: %s",
		retryAfter,
	)
}

// IsMissingScopeError checks if an error is due to missing OAuth scopes.
func IsMissingScopeError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "missing_scope") || strings.Contains(errStr, "not_allowed")
}

// IsNotFoundError checks if an error is a not-found error.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var errWithCode *ErrorWithExitCode
	if errors.As(err, &errWithCode) {
		return errWithCode.ExitCode == ExitNotFound
	}
	return ClassifySlackError(err) == ExitNotFound
}
