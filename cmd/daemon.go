package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	"github.com/kehao95/slack-agent-cli/internal/eventstore"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run and inspect the local Slack event daemon",
	Long:  "Run a foreground Socket Mode daemon that stores normalized Slack events in the local SQLite event cache.",
}

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the event cache daemon",
	Long: `Open a Slack Socket Mode connection and append matching events to the local SQLite cache.

The command runs in the foreground by design so it can be supervised by launchd, systemd, tmux, or an agent runner.`,
	Example: `  # Cache all visible events for 24h
  SLACK_CLI_ROLE=bot slk daemon run

  # Cache only a test channel and skip the bot's own messages
  SLACK_CLI_ROLE=bot slk daemon run --channel "#_bot-testing" --exclude-self`,
	RunE: runDaemonRun,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show local event cache status",
	RunE:  runDaemonStatus,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStatusCmd)

	daemonRunCmd.Flags().String("channel", "", "Restrict caching to a single channel/conversation name or ID")
	daemonRunCmd.Flags().String("conversation-type", "", "Filter cached conversation types: channel,private,dm,mpdm,app_home")
	daemonRunCmd.Flags().String("thread", "", "Restrict caching to a specific thread_ts")
	daemonRunCmd.Flags().Bool("threads-only", false, "Only cache thread-related message events")
	daemonRunCmd.Flags().Bool("exclude-self", false, "Do not cache events produced by the active auth identity")
	daemonRunCmd.Flags().Bool("raw", false, "Store the raw Slack payload for each event")
	daemonRunCmd.Flags().Duration("retention", 24*time.Hour, "How long to retain cached events")
}

func runDaemonRun(cmd *cobra.Command, args []string) error {
	cfg, token, cookie, role, configPath, err := loadConfigForEvents()
	if err != nil {
		return err
	}
	cmdCtx, err := NewStreamingCommandContextWithToken(cmd, token, cookie)
	if err != nil {
		return err
	}
	cmdCtx.Config = cfg
	cmdCtx.AuthRole = role
	cmdCtx.AuthToken = token
	cmdCtx.AuthCookie = cookie
	sanitizeRuntimeContextForRole(cmdCtx)
	defer cmdCtx.Close()
	if err := cmdCtx.EnsureAuthIdentity(cmdCtx.Ctx); err != nil {
		return err
	}

	store, err := openEventStoreForContext(configPath, cmdCtx)
	if err != nil {
		return err
	}
	defer store.Close()

	filter, err := buildDaemonStreamFilter(cmd, cmdCtx)
	if err != nil {
		return err
	}
	includeRaw, _ := cmd.Flags().GetBool("raw")
	retention, _ := cmd.Flags().GetDuration("retention")
	if retention <= 0 {
		retention = 24 * time.Hour
	}

	fmt.Fprintf(os.Stderr, "Caching Slack events in %s (retention %s)\n", store.Path(), retention)
	return runEventCacheLoop(cmd, cmdCtx, store, filter, includeRaw, retention)
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	_, configPath, err := config.Load(cfgFile)
	if err != nil {
		return err
	}
	cmdCtx, err := NewCommandContext(cmd, 10*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	store, err := openEventStoreForContext(configPath, cmdCtx)
	if err != nil {
		return err
	}
	defer store.Close()

	count, err := store.Count(cmdCtx.Ctx)
	if err != nil {
		return err
	}
	cursor, err := store.LatestCursor(cmdCtx.Ctx)
	if err != nil {
		return err
	}
	status := map[string]interface{}{
		"ok":            true,
		"team_id":       cmdCtx.TeamID,
		"db_path":       store.Path(),
		"event_count":   count,
		"latest_cursor": cursor,
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(status)
}

func openEventStoreForContext(configPath string, cmdCtx *CommandContext) (*eventstore.Store, error) {
	dbPath, err := eventstore.DefaultPath(configPath, cmdCtx.TeamID)
	if err != nil {
		return nil, fmt.Errorf("resolve event store path: %w", err)
	}
	store, err := eventstore.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open event store: %w", err)
	}
	return store, nil
}

func buildDaemonStreamFilter(cmd *cobra.Command, cmdCtx *CommandContext) (streamFilter, error) {
	channelInput, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if strings.TrimSpace(channelInput) != "" {
		resolved, err := cmdCtx.ResolveChannel(channelInput)
		if err != nil {
			return streamFilter{}, err
		}
		channelID = resolved
	}
	conversationTypeArg, _ := cmd.Flags().GetString("conversation-type")
	conversationTypes, err := parseConversationTypes(conversationTypeArg)
	if err != nil {
		return streamFilter{}, err
	}
	threadTS, _ := cmd.Flags().GetString("thread")
	threadsOnly, _ := cmd.Flags().GetBool("threads-only")
	excludeSelf, _ := cmd.Flags().GetBool("exclude-self")
	return streamFilter{
		ChannelID:         channelID,
		ConversationTypes: conversationTypes,
		ThreadTS:          strings.TrimSpace(threadTS),
		ThreadsOnly:       threadsOnly,
		ExcludeSelf:       excludeSelf,
	}, nil
}

func runEventCacheLoop(cmd *cobra.Command, cmdCtx *CommandContext, store *eventstore.Store, filter streamFilter, includeRaw bool, retention time.Duration) error {
	normalizer := newEventNormalizer(cmdCtx)
	socketClient := slack.NewSocketModeClient(cmdCtx.AuthToken, cmdCtx.AuthCookie, cmdCtx.Config.AppToken)
	pruneTicker := time.NewTicker(time.Minute)
	defer pruneTicker.Stop()

	if _, err := store.PruneOlderThan(cmdCtx.Ctx, time.Now().Add(-retention)); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- socketClient.RunContext(cmdCtx.Ctx)
	}()

	for {
		select {
		case <-cmdCtx.Ctx.Done():
			return nil
		case <-pruneTicker.C:
			if _, err := store.PruneOlderThan(cmdCtx.Ctx, time.Now().Add(-retention)); err != nil {
				fmt.Fprintf(os.Stderr, "failed to prune event cache: %v\n", err)
			}
		case err := <-errCh:
			if err == nil || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		case evt, ok := <-socketClient.Events:
			if !ok {
				return nil
			}
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				fmt.Fprintln(os.Stderr, "Connecting to Slack Socket Mode...")
			case socketmode.EventTypeConnected:
				fmt.Fprintln(os.Stderr, "Connected to Slack Socket Mode.")
			case socketmode.EventTypeConnectionError:
				fmt.Fprintln(os.Stderr, "Slack Socket Mode connection error. Waiting for reconnect...")
			case socketmode.EventTypeEventsAPI:
				if evt.Request != nil {
					socketClient.Ack(*evt.Request)
				}
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				normalized, emit, err := normalizer.Normalize(eventsAPIEvent, evt.Request, includeRaw)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to normalize event: %v\n", err)
					continue
				}
				if !emit || !filter.Match(normalized) {
					continue
				}
				cursor, err := store.Insert(cmdCtx.Ctx, streamEventToStore(normalized))
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to cache event: %v\n", err)
					continue
				}
				fmt.Fprintf(os.Stderr, "cached event cursor=%d type=%s channel=%s ts=%s\n", cursor, normalized.Type, normalized.ChannelID, normalized.TS)
			}
		}
	}
}
