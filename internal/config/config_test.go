package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDefaultWhenMissing(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLI_FORMAT", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")

	path := filepath.Join(t.TempDir(), "config.json")
	cfg, actualPath, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if actualPath != path {
		t.Fatalf("expected path %s, got %s", path, actualPath)
	}
	if cfg.UserToken != "" {
		t.Fatalf("expected empty user token, got %q", cfg.UserToken)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLI_FORMAT", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")

	path := filepath.Join(t.TempDir(), "slack", "config.json")
	cfg := DefaultConfig()
	cfg.UserToken = "xoxp-123"
	savedPath, err := Save(path, cfg)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if savedPath != path {
		t.Fatalf("expected saved path %s, got %s", path, savedPath)
	}
	loaded, actualPath, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if actualPath != path {
		t.Fatalf("expected actual path %s, got %s", path, actualPath)
	}
	if loaded.UserToken != cfg.UserToken {
		t.Fatalf("expected user token %s, got %s", cfg.UserToken, loaded.UserToken)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "xoxp-env")
	t.Setenv("SLACK_CLI_FORMAT", "json")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.UserToken != "xoxp-env" {
		t.Fatalf("expected user token override, got %s", cfg.UserToken)
	}
	if cfg.Defaults.OutputFormat != "json" {
		t.Fatalf("expected format override json, got %s", cfg.Defaults.OutputFormat)
	}
}

func TestApplyEnvOverridesClientToken(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "xoxc-client")
	t.Setenv("SLACK_CLIENT_COOKIE", "xoxd-cookie")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.UserToken != "xoxc-client" {
		t.Fatalf("expected client token override, got %s", cfg.UserToken)
	}
	if cfg.Cookie != "xoxd-cookie" {
		t.Fatalf("expected cookie override, got %s", cfg.Cookie)
	}
}

func TestApplyEnvOverridesUserTokenTakesPrecedence(t *testing.T) {
	// When both are set, SLACK_USER_TOKEN should win
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user")
	t.Setenv("SLACK_CLIENT_TOKEN", "xoxc-client")
	t.Setenv("SLACK_CLIENT_COOKIE", "xoxd-cookie")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.UserToken != "xoxp-user" {
		t.Fatalf("expected SLACK_USER_TOKEN to take precedence, got %s", cfg.UserToken)
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error when user token empty")
	}
	cfg.UserToken = "xoxp"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
