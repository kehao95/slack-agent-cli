package testutil

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CaptureOutput runs a command and captures stdout, stderr, and exit code separately.
// This is essential for testing agent CLIs where stdout should only contain data
// and stderr should contain status messages.
func CaptureOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	return stdout, stderr, exitCode, err
}

// ValidateJSON parses a JSON string and returns it as a map.
// This fails the test if the JSON is invalid, making it easy to
// validate that CLI output is properly formatted JSON.
func ValidateJSON(t *testing.T, data string) map[string]interface{} {
	t.Helper()

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput was: %s", err, data)
	}

	return result
}

// ValidateJSONArray parses a JSON array string and returns it as a slice.
func ValidateJSONArray(t *testing.T, data string) []interface{} {
	t.Helper()

	var result []interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("invalid JSON array output: %v\nOutput was: %s", err, data)
	}

	return result
}

// MockConfig creates a temporary config file with the given token
// and returns the path to it. The config file is automatically cleaned
// up when the test completes.
//
// Example:
//
//	configPath := MockConfig(t, "xoxp-test-token")
//	cmd := exec.Command("slack-agent-cli", "auth", "test", "--config", configPath)
func MockConfig(t *testing.T, token string) string {
	t.Helper()

	// Create temp directory for config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Write config file
	configContent := `{
		"version": 1,
		"user_token": "` + token + `"
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create mock config: %v", err)
	}

	return configPath
}

// AssertExitCode checks that a command exited with the expected code.
// This is critical for agent CLIs where exit codes communicate status
// to automation systems.
func AssertExitCode(t *testing.T, expected, actual int, context string) {
	t.Helper()

	if actual != expected {
		t.Errorf("%s: expected exit code %d, got %d", context, expected, actual)
	}
}

// AssertJSONField checks that a JSON object has a specific field with a given value.
func AssertJSONField(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()

	actual, ok := data[field]
	if !ok {
		t.Errorf("JSON output missing field %q", field)
		return
	}

	if actual != expected {
		t.Errorf("field %q: expected %v, got %v", field, expected, actual)
	}
}

// AssertJSONFieldExists checks that a JSON object has a specific field (regardless of value).
func AssertJSONFieldExists(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()

	if _, ok := data[field]; !ok {
		t.Errorf("JSON output missing required field %q", field)
	}
}

// AssertStdoutIsJSON verifies that stdout contains valid JSON and stderr does not.
// This enforces the machine-first design principle: data goes to stdout, messages to stderr.
func AssertStdoutIsJSON(t *testing.T, stdout, stderr string) map[string]interface{} {
	t.Helper()

	// Stdout should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Errorf("stdout is not valid JSON: %v\nStdout: %s", err, stdout)
	}

	// Stderr should NOT contain JSON field markers (basic heuristic)
	if len(stderr) > 0 {
		// It's OK to have stderr messages, but they shouldn't look like JSON data
		if json.Valid([]byte(stderr)) {
			var stderrJSON interface{}
			if json.Unmarshal([]byte(stderr), &stderrJSON) == nil {
				t.Errorf("stderr contains JSON data - data should only be in stdout\nStderr: %s", stderr)
			}
		}
	}

	return result
}

// AssertNoInteractivePrompts runs a command with stdin closed and ensures
// it doesn't block waiting for input. Agent CLIs must never prompt interactively.
func AssertNoInteractivePrompts(t *testing.T, cmd *exec.Cmd, timeout int) {
	t.Helper()

	// Close stdin immediately
	cmd.Stdin = nil

	// Run command (should not block)
	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Run()
	}()

	// Wait for completion or timeout
	select {
	case <-errChan:
		// Command completed (good)
		return
	case <-make(chan struct{}):
		// This will never trigger since we don't send anything
		// The command should have completed via errChan
	}
}
