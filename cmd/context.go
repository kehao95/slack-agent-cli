package cmd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/cache"
	"github.com/kehao95/slack-agent-cli/internal/channels"
	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/kehao95/slack-agent-cli/internal/usergroups"
	"github.com/kehao95/slack-agent-cli/internal/users"
	"github.com/spf13/cobra"
)

// CommandContext encapsulates common dependencies for command handlers.
// It eliminates boilerplate setup code that was previously duplicated
// across 20+ command handlers.
type CommandContext struct {
	Ctx               context.Context
	Cancel            context.CancelFunc
	Config            *config.Config
	TeamID            string
	AuthRole          string
	AuthToken         string
	AuthCookie        string
	AuthUserID        string
	AuthBotID         string
	Client            *slack.APIClient
	CacheStore        *cache.Store
	ChannelResolver   *channels.Resolver
	UserResolver      *users.Resolver
	UserGroupResolver *usergroups.Resolver
}

// NewCommandContext initializes all common dependencies needed by commands.
// Pass timeout=0 to use the default timeout of 30 seconds.
func NewCommandContext(cmd *cobra.Command, timeout time.Duration) (*CommandContext, error) {
	return newCommandContext(cmd, timeout, false, "", "", true)
}

// NewStreamingCommandContext initializes command dependencies without applying a deadline to the
// command context, while still using a bounded setup call for auth/team discovery.
func NewStreamingCommandContext(cmd *cobra.Command) (*CommandContext, error) {
	return newCommandContext(cmd, 0, true, "", "", true)
}

// NewStreamingCommandContextWithToken initializes streaming command dependencies using an explicit
// API token while preserving the loaded config for app-level settings.
func NewStreamingCommandContextWithToken(cmd *cobra.Command, token, cookie string) (*CommandContext, error) {
	return newCommandContext(cmd, 0, true, token, cookie, false)
}

func newCommandContext(cmd *cobra.Command, timeout time.Duration, noTimeout bool, tokenOverride, cookieOverride string, validateConfig bool) (*CommandContext, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return nil, errors.ConfigError("failed to load config: %w", err)
	}
	if validateConfig {
		if err := cfg.Validate(); err != nil {
			return nil, errors.ConfigError("invalid config (%s): %w", path, err)
		}
	}
	if !validateConfig && strings.TrimSpace(tokenOverride) == "" {
		return nil, errors.ConfigError("invalid config (%s): token is required", path)
	}

	apiToken, apiCookie, authRole, err := cfg.ActiveAuth()
	if err != nil && validateConfig {
		return nil, errors.ConfigError("invalid config (%s): %w", path, err)
	}
	if tokenOverride != "" {
		apiToken = tokenOverride
		apiCookie = cookieOverride
		authRole = "override"
	}

	client := slack.NewAuto(apiToken, apiCookie)
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if noTimeout {
		ctx, cancel = context.WithCancel(cmd.Context())
	} else {
		ctx, cancel = context.WithTimeout(cmd.Context(), timeout)
	}

	setupCtx, setupCancel := context.WithTimeout(cmd.Context(), timeout)
	defer setupCancel()

	authInfo, err := resolveAuthInfo(setupCtx, client)
	if err != nil {
		cancel()
		return nil, err
	}
	authInfo = authInfoForRole(authInfo, authRole)
	sanitizeRuntimeConfigForRole(cfg, authRole)

	cacheStore, err := cache.DefaultStore(authInfo.TeamID)
	if err != nil {
		cancel()
		return nil, errors.ConfigError("failed to initialize cache: %w", err)
	}

	return &CommandContext{
		Ctx:               ctx,
		Cancel:            cancel,
		Config:            cfg,
		TeamID:            authInfo.TeamID,
		AuthRole:          authRole,
		AuthToken:         apiToken,
		AuthCookie:        apiCookie,
		AuthUserID:        authInfo.UserID,
		AuthBotID:         authInfo.BotID,
		Client:            client,
		CacheStore:        cacheStore,
		ChannelResolver:   channels.NewCachedResolver(client, cacheStore),
		UserResolver:      users.NewCachedResolver(client, cacheStore),
		UserGroupResolver: usergroups.NewCachedResolver(client, cacheStore),
	}, nil
}

// NewCommandContextWithToken creates a minimal context with a provided token.
// This is useful for verifying tokens before saving them to config.
// It does not initialize cache or resolvers since those require team ID.
func NewCommandContextWithToken(cmd *cobra.Command, timeout time.Duration, token string) (*CommandContext, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := slack.NewAuto(token, "")
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)

	return &CommandContext{
		Ctx:       ctx,
		Cancel:    cancel,
		AuthRole:  "override",
		AuthToken: token,
		Client:    client,
	}, nil
}

// Close releases resources held by the CommandContext.
// Always defer Close() after creating a CommandContext.
func (c *CommandContext) Close() {
	if c.Cancel != nil {
		c.Cancel()
	}
}

// ResolveChannel converts a channel name or ID to a channel ID.
// Convenience method that wraps ChannelResolver.ResolveID.
func (c *CommandContext) ResolveChannel(input string) (string, error) {
	return c.ChannelResolver.ResolveID(c.Ctx, input)
}

// EnsureAuthIdentity fills in the active Slack user/bot IDs when the context was created with
// SLACK_TEAM_ID and skipped auth.test during setup.
func (c *CommandContext) EnsureAuthIdentity(ctx context.Context) error {
	if c == nil || c.Client == nil || c.hasAuthIdentity() {
		return nil
	}
	resp, err := c.Client.AuthTest(ctx)
	if err != nil {
		return err
	}
	resp = authInfoForRole(resp, c.AuthRole)
	if c.TeamID == "" {
		c.TeamID = resp.TeamID
	}
	c.AuthUserID = resp.UserID
	c.AuthBotID = resp.BotID
	return nil
}

func (c *CommandContext) hasAuthIdentity() bool {
	if c == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(c.AuthRole)) {
	case config.RoleUser:
		return strings.TrimSpace(c.AuthUserID) != ""
	case config.RoleBot:
		return strings.TrimSpace(c.AuthUserID) != "" || strings.TrimSpace(c.AuthBotID) != ""
	default:
		return strings.TrimSpace(c.AuthUserID) != "" && strings.TrimSpace(c.AuthBotID) != ""
	}
}

func authInfoForRole(authInfo *slack.AuthTestResponse, role string) *slack.AuthTestResponse {
	if authInfo == nil {
		return nil
	}
	if strings.ToLower(strings.TrimSpace(role)) == config.RoleUser {
		authInfo.BotID = ""
	}
	return authInfo
}

func sanitizeRuntimeConfigForRole(cfg *config.Config, role string) {
	if cfg == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case config.RoleUser:
		cfg.BotToken = ""
	case config.RoleBot:
		cfg.UserToken = ""
		cfg.Cookie = ""
	}
}

func sanitizeRuntimeContextForRole(cmdCtx *CommandContext) {
	if cmdCtx == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(cmdCtx.AuthRole)) == config.RoleUser {
		cmdCtx.AuthBotID = ""
	}
	sanitizeRuntimeConfigForRole(cmdCtx.Config, cmdCtx.AuthRole)
}

func resolveAuthInfo(ctx context.Context, client *slack.APIClient) (*slack.AuthTestResponse, error) {
	if envTeamID := strings.TrimSpace(os.Getenv("SLACK_TEAM_ID")); envTeamID != "" {
		return &slack.AuthTestResponse{TeamID: envTeamID}, nil
	}
	resp, err := client.AuthTest(ctx)
	if err != nil {
		return nil, errors.AuthError("auth test failed: %w", err)
	}
	if resp.TeamID == "" {
		return nil, errors.AuthError("auth test missing team id")
	}
	return resp, nil
}
