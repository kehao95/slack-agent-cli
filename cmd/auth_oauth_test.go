package cmd

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kehao95/slack-agent-cli/internal/config"
)

func TestOAuthStateIsRandomAndOneTime(t *testing.T) {
	first, err := newOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	if first.value == "" || first.value == second.value {
		t.Fatalf("OAuth states are empty or equal: %q %q", first.value, second.value)
	}
	if !first.validateAndConsume(first.value) {
		t.Fatal("valid state was rejected")
	}
	if first.validateAndConsume(first.value) {
		t.Fatal("reused state was accepted")
	}
}

func TestBuildAuthURLIncludesStateAndManifestScopes(t *testing.T) {
	raw := buildAuthURL("client", "https://example.com/callback", defaultOAuthUserScopes, defaultOAuthBotScopes, "state-value")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("state") != "state-value" || query.Get("redirect_uri") != "https://example.com/callback" {
		t.Fatalf("unexpected auth URL: %s", raw)
	}
	for _, scope := range []string{"channels:write", "files:write", "lists:read", "emoji:read"} {
		if !strings.Contains(query.Get("user_scope"), scope) {
			t.Errorf("default scopes missing %s", scope)
		}
	}
	for _, scope := range []string{"channels:manage", "chat:write", "usergroups:write", "files:write"} {
		if !strings.Contains(query.Get("scope"), scope) {
			t.Errorf("default bot scopes missing %s", scope)
		}
	}
}

func TestOAuthCallbackRejectsInvalidStateBeforeExchange(t *testing.T) {
	state, err := newOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	exchanged := false
	request := httptest.NewRequest("GET", "/callback?code=secret-code&state=wrong", nil)
	recorder := httptest.NewRecorder()
	handleOAuthCallback(recorder, request, oauthCallbackOptions{
		State: state,
		Exchange: func(_, _, _, _ string) (*OAuthTokenResponse, error) {
			exchanged = true
			return nil, nil
		},
	})
	if recorder.Code != 400 || exchanged || !strings.Contains(recorder.Body.String(), "invalid_state") {
		t.Fatalf("code=%d exchanged=%v body=%s", recorder.Code, exchanged, recorder.Body.String())
	}
}

func TestOAuthCallbackSavesBothTokensWithoutReturningThem(t *testing.T) {
	for _, env := range []string{"SLACK_USER_TOKEN", "SLACK_BOT_TOKEN", "SLACK_CLI_ROLE", "SLACK_CLIENT_TOKEN", "SLACK_CLIENT_COOKIE"} {
		t.Setenv(env, "")
	}
	state, err := newOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	request := httptest.NewRequest("GET", "/callback?code=authorization-code&state="+url.QueryEscape(state.value), nil)
	recorder := httptest.NewRecorder()
	var logs bytes.Buffer
	handleOAuthCallback(recorder, request, oauthCallbackOptions{
		ClientID: "client", ClientSecret: "client-secret", RedirectURI: "https://example.com/callback",
		State: state, Save: true, ConfigPath: configPath,
		Log: &logs,
		Exchange: func(code, clientID, clientSecret, redirectURI string) (*OAuthTokenResponse, error) {
			if code != "authorization-code" || clientID != "client" || clientSecret != "client-secret" {
				t.Fatalf("unexpected exchange arguments")
			}
			response := &OAuthTokenResponse{OK: true, AccessToken: "xoxb-bot-secret", BotUserID: "B123"}
			response.Team.ID = "T123"
			response.Team.Name = "Example"
			response.AuthedUser.ID = "U123"
			response.AuthedUser.AccessToken = "xoxp-user-secret"
			return response, nil
		},
	})
	if recorder.Code != 200 {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	publicOutput := recorder.Body.String() + logs.String()
	if strings.Contains(publicOutput, "xoxp-") || strings.Contains(publicOutput, "xoxb-") || strings.Contains(publicOutput, "client-secret") || strings.Contains(publicOutput, "authorization-code") {
		t.Fatalf("callback leaked a secret: %s", publicOutput)
	}
	var publicResponse map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &publicResponse); err != nil {
		t.Fatal(err)
	}
	if publicResponse["saved"] != true || publicResponse["user_id"] != "U123" || publicResponse["bot_user_id"] != "B123" {
		t.Fatalf("unexpected public response: %v", publicResponse)
	}

	saved, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.UserToken != "xoxp-user-secret" || saved.BotToken != "xoxb-bot-secret" || saved.Role != config.RoleUser {
		t.Fatalf("tokens not saved correctly: %+v", saved)
	}
}

func TestSaveOAuthTokensSelectsBotRoleWhenNoUserToken(t *testing.T) {
	for _, env := range []string{"SLACK_USER_TOKEN", "SLACK_BOT_TOKEN", "SLACK_CLI_ROLE"} {
		t.Setenv(env, "")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveOAuthTokensToConfig(path, "", "xoxb-only"); err != nil {
		t.Fatal(err)
	}
	saved, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BotToken != "xoxb-only" || saved.Role != config.RoleBot {
		t.Fatalf("unexpected saved config: %+v", saved)
	}
}
