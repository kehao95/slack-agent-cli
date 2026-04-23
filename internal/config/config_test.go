package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDefaultWhenMissing(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLI_FORMAT", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")
	t.Setenv("SLACK_CLI_ROLE", "")

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
	if cfg.Role != RoleUser {
		t.Fatalf("expected default role %q, got %q", RoleUser, cfg.Role)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_CLI_FORMAT", "")
	t.Setenv("SLACK_CLIENT_TOKEN", "")
	t.Setenv("SLACK_CLI_ROLE", "")

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
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-env")
	t.Setenv("SLACK_APP_TOKEN", "xapp-env")
	t.Setenv("SLACK_CLI_ROLE", "bot")
	t.Setenv("SLACK_CLI_FORMAT", "json")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.UserToken != "xoxp-env" {
		t.Fatalf("expected user token override, got %s", cfg.UserToken)
	}
	if cfg.BotToken != "xoxb-env" {
		t.Fatalf("expected bot token override, got %s", cfg.BotToken)
	}
	if cfg.AppToken != "xapp-env" {
		t.Fatalf("expected app token override, got %s", cfg.AppToken)
	}
	if cfg.Role != "bot" {
		t.Fatalf("expected role override bot, got %s", cfg.Role)
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

func TestValidateBotRole(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Role = RoleBot
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error when bot token empty")
	}
	cfg.BotToken = "xoxb"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateInvalidRole(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Role = "admin"
	cfg.UserToken = "xoxp"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid role")
	}
}

func TestActiveAuth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UserToken = "xoxp-user"
	cfg.BotToken = "xoxb-bot"

	token, cookie, role, err := cfg.ActiveAuth()
	if err != nil {
		t.Fatalf("ActiveAuth returned error: %v", err)
	}
	if token != "xoxp-user" || cookie != "" || role != RoleUser {
		t.Fatalf("expected user auth, got token=%q cookie=%q role=%q", token, cookie, role)
	}

	cfg.Role = RoleBot
	token, cookie, role, err = cfg.ActiveAuth()
	if err != nil {
		t.Fatalf("ActiveAuth returned error: %v", err)
	}
	if token != "xoxb-bot" || cookie != "" || role != RoleBot {
		t.Fatalf("expected bot auth, got token=%q cookie=%q role=%q", token, cookie, role)
	}
}
