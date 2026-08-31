package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	loginToken  string
	loginVerify bool
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save authentication token to config",
	Long: `Save a Slack user, bot, or client token to the config file.

User tokens start with 'xoxp-', bot tokens with 'xoxb-', and client tokens with
'xoxc-'. Client tokens also require SLACK_CLIENT_COOKIE when used. Use
'slk auth oauth' to obtain tokens through the OAuth flow, or pass one directly
with --token. Saving a token selects its matching user or bot role.`,
	Example: `  # Save a token to config
  slk auth login --token xoxp-xxx-xxx-xxx

  # Save and verify the token works
  slk auth login --token xoxp-xxx-xxx-xxx --verify`,
	RunE: runAuthLogin,
}

func init() {
	authCmd.AddCommand(authLoginCmd)

	authLoginCmd.Flags().StringVar(&loginToken, "token", "", "Slack token (xoxp-..., xoxb-..., or xoxc-...)")
	authLoginCmd.Flags().BoolVar(&loginVerify, "verify", false, "Verify the token works by calling Slack API")
	authLoginCmd.MarkFlagRequired("token")
}

// LoginResult represents the result of the login command
type LoginResult struct {
	OK         bool   `json:"ok"`
	ConfigPath string `json:"config_path"`
	TokenType  string `json:"token_type"`
	Verified   bool   `json:"verified,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	TeamID     string `json:"team_id,omitempty"`
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// Validate token format
	if err := validateTokenFormat(loginToken); err != nil {
		return err
	}

	// Determine token type for display
	tokenType := "user"
	if strings.HasPrefix(loginToken, "xoxb-") {
		tokenType = "bot"
	} else if strings.HasPrefix(loginToken, "xoxc-") {
		tokenType = "client"
	}

	result := LoginResult{
		OK:        true,
		TokenType: tokenType,
	}

	// Load first so xoxc verification can use an existing cookie. Load applies
	// SLACK_CLIENT_COOKIE as an environment override as well.
	cfg, configPath, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.DefaultConfig()
		configPath, err = config.DefaultPath()
		if err != nil {
			return fmt.Errorf("determine config path: %w", err)
		}
	}

	// Optionally verify token by calling auth.test
	if loginVerify {
		cookie := ""
		if strings.HasPrefix(loginToken, "xoxc-") {
			cookie = strings.TrimSpace(cfg.Cookie)
			if cookie == "" {
				return fmt.Errorf("xoxc client token verification requires SLACK_CLIENT_COOKIE or a cookie in the existing config")
			}
		}
		cmdCtx, err := NewCommandContextWithCredentials(cmd, 10*time.Second, loginToken, cookie)
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}
		defer cmdCtx.Close()

		authResult, err := cmdCtx.Client.AuthTest(cmdCtx.Ctx)
		if err != nil {
			return fmt.Errorf("token verification failed: %w", err)
		}
		result.Verified = true
		result.UserID = authResult.UserID
		result.TeamID = authResult.TeamID
	}

	// Save the token in the slot selected by its type. Keeping bot credentials
	// in user_token makes role=bot unusable and can accidentally select the
	// wrong identity on the next invocation.
	applyLoginToken(cfg, loginToken)

	savedPath, err := config.Save(configPath, cfg)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	result.ConfigPath = savedPath

	// Human-friendly output to stderr
	fmt.Fprintf(os.Stderr, "Token saved to %s\n", savedPath)
	if result.Verified {
		fmt.Fprintf(os.Stderr, "Token verified successfully (user: %s, team: %s)\n", result.UserID, result.TeamID)
	}

	return output.Print(cmd, result)
}

func applyLoginToken(cfg *config.Config, token string) {
	if strings.HasPrefix(token, "xoxb-") {
		cfg.BotToken = token
		cfg.Role = config.RoleBot
		return
	}

	cfg.UserToken = token
	cfg.Role = config.RoleUser
}

func validateTokenFormat(token string) error {
	if token == "" {
		return fmt.Errorf("token is required")
	}

	validPrefixes := []string{"xoxp-", "xoxb-", "xoxc-"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(token, prefix) {
			return nil
		}
	}

	return fmt.Errorf("invalid token format: token should start with xoxp- (user), xoxb- (bot), or xoxc- (client)")
}
