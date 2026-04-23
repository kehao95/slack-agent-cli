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
	AuthRole          string
	AuthToken         string
	AuthCookie        string
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

	teamID, err := resolveTeamID(setupCtx, client)
	if err != nil {
		cancel()
		return nil, err
	}

	cacheStore, err := cache.DefaultStore(teamID)
	if err != nil {
		cancel()
		return nil, errors.ConfigError("failed to initialize cache: %w", err)
	}

	return &CommandContext{
		Ctx:               ctx,
		Cancel:            cancel,
		Config:            cfg,
		AuthRole:          authRole,
		AuthToken:         apiToken,
		AuthCookie:        apiCookie,
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

func resolveTeamID(ctx context.Context, client *slack.APIClient) (string, error) {
	if envTeamID := strings.TrimSpace(os.Getenv("SLACK_TEAM_ID")); envTeamID != "" {
		return envTeamID, nil
	}
	resp, err := client.AuthTest(ctx)
	if err != nil {
		return "", errors.AuthError("auth test failed: %w", err)
	}
	if resp.TeamID == "" {
		return "", errors.AuthError("auth test missing team id")
	}
	return resp.TeamID, nil
}
