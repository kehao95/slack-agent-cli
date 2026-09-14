package policy

import (
	"errors"
	"os"
	"strings"
	"testing"

	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
)

func TestReadOnlyEnvironment(t *testing.T) {
	t.Setenv(EnvReadOnly, "temporary")
	if err := os.Unsetenv(EnvReadOnly); err != nil {
		t.Fatal(err)
	}
	if enabled, err := ReadOnly(); enabled || err != nil {
		t.Fatalf("unset: enabled=%t error=%v", enabled, err)
	}
	for _, value := range []string{"true", "TRUE", "True", "1", "t", "T", "false", "FALSE", "False", "0", "f", "F", "", "yes", " true ", "token-secret"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvReadOnly, value)
			enabled, err := ReadOnly()
			switch value {
			case "true", "TRUE", "True", "1", "t", "T":
				if !enabled || err != nil {
					t.Fatalf("enabled=%t error=%v", enabled, err)
				}
			case "false", "FALSE", "False", "0", "f", "F":
				if enabled || err != nil {
					t.Fatalf("enabled=%t error=%v", enabled, err)
				}
			default:
				var coded *cerrors.ErrorWithExitCode
				if !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitConfig {
					t.Fatalf("expected config error: %v", err)
				}
				if strings.Contains(err.Error(), "token-secret") {
					t.Fatal("configuration value leaked")
				}
			}
		})
	}
}

func TestReviewedMethods(t *testing.T) {
	t.Setenv(EnvReadOnly, "true")
	for method := range readMethods {
		if err := CheckMethod(method); err != nil {
			t.Errorf("read %s: %v", method, err)
		}
	}
	for _, method := range []string{"chat.postMessage", "conversations.open", "conversations.mark", "users.profile.set", "usergroups.users.update", "files.getUploadURLExternal", "files.completeUploadExternal", "files.sharedPublicURL", "oauth.v2.access", "assistant.search.context", "future.read", "future.list", "Conversations.info"} {
		err := CheckMethod(method)
		var violation *ViolationError
		var coded *cerrors.ErrorWithExitCode
		if !errors.As(err, &violation) || violation.Operation != method || !errors.As(err, &coded) || coded.ExitCode != cerrors.ExitPermission {
			t.Errorf("expected stable permission error for %s: %v", method, err)
		}
	}
	t.Setenv(EnvReadOnly, "false")
	if err := CheckMethod("future.write"); err != nil {
		t.Fatalf("default behavior changed: %v", err)
	}
}
