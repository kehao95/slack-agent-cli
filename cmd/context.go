package cmd

import (
	"context"
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
	Client            *slack.APIClient
	CacheStore        *cache.Store
	ChannelResolver   *channels.Resolver
	UserResolver      *users.Resolver
	UserGroupResolver *usergroups.Resolver
}

// NewCommandContext initializes all common dependencies needed by commands.
// Pass timeout=0 to use the default timeout of 30 seconds.
func NewCommandContext(cmd *cobra.Command, timeout time.Duration) (*CommandContext, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return nil, errors.ConfigError("failed to load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, errors.ConfigError("invalid config (%s): %w", path, err)
	}

	cacheStore, err := cache.DefaultStore()
	if err != nil {
		return nil, errors.ConfigError("failed to initialize cache: %w", err)
	}

	client := slack.New(cfg.UserToken)
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)

	return &CommandContext{
		Ctx:               ctx,
		Cancel:            cancel,
		Config:            cfg,
		Client:            client,
		CacheStore:        cacheStore,
		ChannelResolver:   channels.NewCachedResolver(client, cacheStore),
		UserResolver:      users.NewCachedResolver(client, cacheStore),
		UserGroupResolver: usergroups.NewCachedResolver(client, cacheStore),
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
