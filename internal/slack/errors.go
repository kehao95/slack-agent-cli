package slack

import "errors"

// Sentinel errors for programmatic error handling.
// Use errors.Is() to check for these errors.
var (
	// ErrChannelRequired indicates a channel parameter is required but was empty.
	ErrChannelRequired = errors.New("channel is required")

	// ErrTextRequired indicates message text or blocks are required but were empty.
	ErrTextRequired = errors.New("text or blocks is required")

	// ErrTimestampRequired indicates a message timestamp is required but was empty.
	ErrTimestampRequired = errors.New("timestamp is required")

	// ErrEmojiRequired indicates an emoji name is required but was empty.
	ErrEmojiRequired = errors.New("emoji is required")

	// ErrUserRequired indicates a user ID is required but was empty.
	ErrUserRequired = errors.New("user is required")

	// ErrQueryRequired indicates a search query is required but was empty.
	ErrQueryRequired = errors.New("search query is required")

	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrRateLimited indicates the API rate limit was exceeded.
	ErrRateLimited = errors.New("rate limited")

	// ErrUnauthorized indicates authentication failed.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrPermissionDenied indicates insufficient permissions.
	ErrPermissionDenied = errors.New("permission denied")
)
