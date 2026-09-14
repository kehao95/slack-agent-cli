// Package policy defines the CLI guardrail against remote Slack mutations.
package policy

import (
	"os"
	"strconv"

	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
)

const EnvReadOnly = "SLACK_CLI_READ_ONLY"

// ReadOnly parses the environment on every invocation. An explicitly empty or
// malformed value is a configuration error; no config/flag can weaken true.
func ReadOnly() (bool, error) {
	value, present := os.LookupEnv(EnvReadOnly)
	if !present {
		return false, nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, cerrors.ConfigError("%s must be a boolean (true or false)", EnvReadOnly)
	}
	return enabled, nil
}

// ViolationError identifies a denied operation without storing request data.
type ViolationError struct{ Operation string }

func (e *ViolationError) Error() string {
	return "read_only_violation: blocked operation " + e.Operation
}

// Deny constructs a stable permission error for a known operation label.
func Deny(operation string) error {
	return &cerrors.ErrorWithExitCode{Err: &ViolationError{Operation: operation}, ExitCode: cerrors.ExitPermission}
}

// CheckMethod uses an exact reviewed allowlist, never HTTP verbs or name
// suffixes. Unsupported and future methods fail closed in read-only mode.
func CheckMethod(method string) error {
	enabled, err := ReadOnly()
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if _, allowed := readMethods[method]; !allowed {
		return Deny(method)
	}
	return nil
}

// Each entry was reviewed as a read operation against its Slack reference:
// https://docs.slack.dev/reference/methods/<method>/
// apps.connections.open is the explicit event-delivery transport exception:
// it obtains a Socket Mode URL; CLI acknowledgments carry no response payload.
// assistant.search.context remains unsupported pending the separate UAT issue.
var readMethods = map[string]struct{}{
	"auth.test":          {},
	"conversations.list": {}, "conversations.info": {}, "conversations.members": {},
	"conversations.history": {}, "conversations.replies": {},
	"users.info": {}, "users.list": {}, "users.lookupByEmail": {},
	"users.profile.get": {}, "users.getPresence": {}, "users.conversations": {},
	"usergroups.list": {}, "usergroups.users.list": {},
	"emoji.list": {}, "pins.list": {}, "reactions.get": {}, "reactions.list": {},
	"files.list": {}, "files.info": {},
	"chat.getPermalink": {}, "chat.scheduledMessages.list": {},
	"search.all": {}, "search.messages": {}, "search.files": {},
	"slackLists.items.list": {}, "slackLists.items.info": {},
	"apps.connections.open": {},
}
