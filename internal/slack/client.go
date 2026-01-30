package slack

import (
	slackapi "github.com/slack-go/slack"
)

// APIClient implements Client by wrapping slack-go's Client.
type APIClient struct {
	sdk *slackapi.Client
}

// New creates a new APIClient using the provided user token.
func New(userToken string) *APIClient {
	return &APIClient{sdk: slackapi.New(userToken)}
}
