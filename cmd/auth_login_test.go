package cmd

import (
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/config"
)

func TestApplyLoginTokenSelectsCredentialRole(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		wantRole string
		wantUser string
		wantBot  string
	}{
		{name: "user", token: "xoxp-user", wantRole: config.RoleUser, wantUser: "xoxp-user"},
		{name: "client", token: "xoxc-client", wantRole: config.RoleUser, wantUser: "xoxc-client"},
		{name: "bot", token: "xoxb-bot", wantRole: config.RoleBot, wantBot: "xoxb-bot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			applyLoginToken(cfg, tt.token)
			if cfg.Role != tt.wantRole || cfg.UserToken != tt.wantUser || cfg.BotToken != tt.wantBot {
				t.Fatalf("got role=%q user=%q bot=%q", cfg.Role, cfg.UserToken, cfg.BotToken)
			}
		})
	}
}
