package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

func TestReadAPIDataInlineStdinAndFile(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString(`{"channel":"C123"}`))

	stdinPayload, err := readAPIData(cmd, "-")
	if err != nil || stdinPayload["channel"] != "C123" {
		t.Fatalf("stdin payload=%v err=%v", stdinPayload, err)
	}
	inlinePayload, err := readAPIData(cmd, `{"limit":100}`)
	if err != nil || inlinePayload["limit"].(interface{ String() string }).String() != "100" {
		t.Fatalf("inline payload=%v err=%v", inlinePayload, err)
	}

	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(`{"user":"U123"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	filePayload, err := readAPIData(cmd, "@"+path)
	if err != nil || filePayload["user"] != "U123" {
		t.Fatalf("file payload=%v err=%v", filePayload, err)
	}
}

func TestReadAPIDataRejectsInvalidInputs(t *testing.T) {
	cmd := &cobra.Command{}
	for _, source := range []string{"", "[]", "null", `{"ok":true} trailing`, "@"} {
		if _, err := readAPIData(cmd, source); err == nil {
			t.Errorf("readAPIData(%q) unexpectedly succeeded", source)
		}
	}
}

func TestAPIMethodValidation(t *testing.T) {
	valid := []string{"api.test", "conversations.history", "slackLists.items.list", "admin.conversations.restrictAccess.addGroup"}
	for _, method := range valid {
		if slack.ValidateAPIMethod(method) != nil {
			t.Errorf("valid method rejected: %s", method)
		}
	}
	invalid := []string{"api", "../api.test", "https://example.com", "api/test", "api.test?x=1"}
	for _, method := range invalid {
		if slack.ValidateAPIMethod(method) == nil {
			t.Errorf("invalid method accepted: %s", method)
		}
	}
}
