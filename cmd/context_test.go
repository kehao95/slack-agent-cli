package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/spf13/cobra"
)

// TestNewCommandContext_ValidConfig verifies that NewCommandContext initializes
// all dependencies correctly when given a valid config.
func TestNewCommandContext_ValidConfig(t *testing.T) {
	// Set up valid config file
	configPath := setupValidConfig(t)
	cfgFile = configPath

	// Create a minimal cobra command
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.SetContext(context.Background())

	// Create command context
	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		t.Fatalf("NewCommandContext returned error: %v", err)
	}
	defer cmdCtx.Close()

	// Verify all fields are initialized
	if cmdCtx.Ctx == nil {
		t.Error("Ctx is nil")
	}
	if cmdCtx.Cancel == nil {
		t.Error("Cancel is nil")
	}
	if cmdCtx.Config == nil {
		t.Error("Config is nil")
	}
	if cmdCtx.Client == nil {
		t.Error("Client is nil")
	}
	if cmdCtx.CacheStore == nil {
		t.Error("CacheStore is nil")
	}
	if cmdCtx.ChannelResolver == nil {
		t.Error("ChannelResolver is nil")
	}
	if cmdCtx.UserResolver == nil {
		t.Error("UserResolver is nil")
	}
	if cmdCtx.UserGroupResolver == nil {
		t.Error("UserGroupResolver is nil")
	}

	// Verify config was loaded correctly
	if cmdCtx.Config.UserToken != "xoxp-test-token" {
		t.Errorf("expected user token 'xoxp-test-token', got '%s'", cmdCtx.Config.UserToken)
	}
}

// TestNewCommandContext_MissingConfig verifies that NewCommandContext returns
// an error when the config file doesn't exist and no env vars are set.
func TestNewCommandContext_MissingConfig(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")

	// Point to non-existent config
	cfgFile = filepath.Join(t.TempDir(), "nonexistent.json")

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	// NewCommandContext should succeed (Load creates default config)
	// but validation should fail
	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err == nil {
		defer cmdCtx.Close()
		t.Fatal("NewCommandContext should return error for invalid config")
	}

	// Error should mention invalid config
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// TestNewCommandContext_InvalidConfig verifies that NewCommandContext returns
// an error when the config is invalid (missing user token).
func TestNewCommandContext_InvalidConfig(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")

	// Create config without user token
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultConfig()
	cfg.UserToken = "" // Invalid: missing token
	_, err := config.Save(configPath, cfg)
	if err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	// Should fail validation
	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err == nil {
		defer cmdCtx.Close()
		t.Fatal("NewCommandContext should return error for invalid config")
	}

	// Error should mention validation
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// TestNewCommandContext_DefaultTimeout verifies that passing timeout=0
// uses the default 30-second timeout.
func TestNewCommandContext_DefaultTimeout(t *testing.T) {
	configPath := setupValidConfig(t)
	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	// Pass timeout=0 to use default
	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		t.Fatalf("NewCommandContext returned error: %v", err)
	}
	defer cmdCtx.Close()

	// Verify context has a deadline
	deadline, ok := cmdCtx.Ctx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline")
	}

	// The deadline should be approximately 30 seconds from now
	// (allow some tolerance for test execution time)
	expectedDeadline := time.Now().Add(30 * time.Second)
	diff := deadline.Sub(expectedDeadline)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("expected deadline around %v, got %v (diff: %v)", expectedDeadline, deadline, diff)
	}
}

// TestNewCommandContext_CustomTimeout verifies that passing a custom timeout
// sets the correct deadline on the context.
func TestNewCommandContext_CustomTimeout(t *testing.T) {
	configPath := setupValidConfig(t)
	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	// Use custom 10-second timeout
	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	customTimeout := 10 * time.Second
	cmdCtx, err := NewCommandContext(cmd, customTimeout)
	if err != nil {
		t.Fatalf("NewCommandContext returned error: %v", err)
	}
	defer cmdCtx.Close()

	// Verify context has a deadline
	deadline, ok := cmdCtx.Ctx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline")
	}

	// The deadline should be approximately 10 seconds from now
	expectedDeadline := time.Now().Add(customTimeout)
	diff := deadline.Sub(expectedDeadline)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("expected deadline around %v, got %v (diff: %v)", expectedDeadline, deadline, diff)
	}
}

// TestCommandContext_Close verifies that Close can be called multiple times
// safely without panicking.
func TestCommandContext_Close(t *testing.T) {
	configPath := setupValidConfig(t)
	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	t.Setenv("SLACK_TEAM_ID", "T123TEST")
	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		t.Fatalf("NewCommandContext returned error: %v", err)
	}

	// Call Close multiple times - should not panic
	cmdCtx.Close()
	cmdCtx.Close()
	cmdCtx.Close()

	// Verify context was cancelled
	select {
	case <-cmdCtx.Ctx.Done():
		// Expected: context should be cancelled
	default:
		t.Error("context should be cancelled after Close()")
	}
}

// TestCommandContext_ResolveChannel verifies the convenience method works.
func TestCommandContext_ResolveChannel(t *testing.T) {
	configPath := setupValidConfig(t)
	cfgFile = configPath

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		t.Fatalf("NewCommandContext returned error: %v", err)
	}
	defer cmdCtx.Close()

	// Test with channel ID (should pass through)
	channelID, err := cmdCtx.ResolveChannel("C123ABC")
	if err != nil {
		t.Errorf("ResolveChannel returned error for valid ID: %v", err)
	}
	if channelID != "C123ABC" {
		t.Errorf("expected 'C123ABC', got '%s'", channelID)
	}

	// Test with empty input (should error)
	_, err = cmdCtx.ResolveChannel("")
	if err == nil {
		t.Error("ResolveChannel should return error for empty input")
	}
}

// TestCommandContext_CloseNilCancel verifies that Close handles nil Cancel gracefully.
func TestCommandContext_CloseNilCancel(t *testing.T) {
	// Create a CommandContext with nil Cancel
	cmdCtx := &CommandContext{
		Cancel: nil,
	}

	// Should not panic
	cmdCtx.Close()
}

// setupValidConfig creates a valid config file for testing and returns its path.
func setupValidConfig(t *testing.T) string {
	t.Helper()
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")

	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultConfig()
	cfg.UserToken = "xoxp-test-token"
	_, err := config.Save(configPath, cfg)
	if err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}
	return configPath
}
