package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/config"
	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Event stream operations",
	Long: `Consume, cache, and query Slack Events API traffic for low-level event workflows.

Best practice for agents:
  1) run slk daemon run to keep a local event queue
  2) process one item at a time with slk events claim
  3) call slk events ack <cursor> after successful processing`,
}

var eventsStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream Slack events over Socket Mode",
	Long: `Open a Socket Mode connection and emit one JSON event per line on stdout.

This command is blocking by design, similar to tail -f.
Connection status and reconnect messages are written to stderr.`,
	Example: `  # Stream all visible message events
  slk events stream

  # Stream only direct messages visible to the bot user
  slk events stream --conversation-type dm

  # Stream one channel
  slk events stream --channel "#support"

  # Stream only message events from one channel
  slk events stream --channel "#support" --event-type message

  # Stream multiple event types
  slk events stream --channel "#support" --event-type message,reaction_added

  # Stream one thread
  slk events stream --channel "#support" --thread "1705312365.000100"

  # Include raw Slack payloads for debugging
  slk events stream --raw`,
	RunE: runEventsStream,
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.AddCommand(eventsStreamCmd)

	addEventsStreamFlags(eventsStreamCmd)
}

func addEventsStreamFlags(cmd *cobra.Command) {
	cmd.Flags().String("channel", "", "Restrict to a single channel/conversation name or ID")
	cmd.Flags().String("conversation-type", "", "Filter by conversation types: channel,private,dm,mpdm,app_home")
	cmd.Flags().String("event-type", "", "Restrict to Slack event types, comma-separated (for example message,reaction_added)")
	cmd.Flags().String("thread", "", "Restrict to a specific thread_ts")
	cmd.Flags().StringP("file", "f", "", "Also append each matching event to this file (open/write/close per event)")
	cmd.Flags().Bool("threads-only", false, "Only emit thread-related message events")
	cmd.Flags().Bool("exclude-self", false, "Exclude events produced by the active auth identity")
	cmd.Flags().Bool("raw", false, "Include the raw Slack payload in each emitted event")
}

func loadConfigForEvents() (*config.Config, string, string, string, string, error) {
	cfg, path, err := config.Load(cfgFile)
	if err != nil {
		return nil, "", "", "", "", cerrors.ConfigError("failed to load config: %w", err)
	}
	token, cookie, role, err := cfg.ActiveAuth()
	if err != nil {
		return nil, "", "", "", "", cerrors.ConfigError("invalid config (%s): %w", path, err)
	}
	if strings.TrimSpace(cfg.AppToken) == "" {
		return nil, "", "", "", "", cerrors.ConfigError("missing app token: set SLACK_APP_TOKEN or add app_token to config")
	}
	return cfg, token, cookie, role, path, nil
}

func buildEventsStreamFilter(cmd *cobra.Command, resolveChannel func(string) (string, error)) (streamFilter, error) {
	channelInput, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if strings.TrimSpace(channelInput) != "" {
		if resolveChannel == nil {
			channelID = strings.TrimSpace(channelInput)
		} else {
			resolved, err := resolveChannel(channelInput)
			if err != nil {
				return streamFilter{}, err
			}
			channelID = resolved
		}
	}

	conversationTypeArg, _ := cmd.Flags().GetString("conversation-type")
	conversationTypes, err := parseConversationTypes(conversationTypeArg)
	if err != nil {
		return streamFilter{}, err
	}

	eventTypeArg, _ := cmd.Flags().GetString("event-type")
	eventTypes, err := parseEventTypes(eventTypeArg)
	if err != nil {
		return streamFilter{}, err
	}

	threadTS, _ := cmd.Flags().GetString("thread")
	threadsOnly, _ := cmd.Flags().GetBool("threads-only")
	excludeSelf, _ := cmd.Flags().GetBool("exclude-self")
	if err := validateThreadsOnlyEventTypes(threadsOnly, eventTypes); err != nil {
		return streamFilter{}, err
	}

	return streamFilter{
		ChannelID:         channelID,
		ConversationTypes: conversationTypes,
		EventTypes:        eventTypes,
		ThreadTS:          strings.TrimSpace(threadTS),
		ThreadsOnly:       threadsOnly,
		ExcludeSelf:       excludeSelf,
	}, nil
}

func runEventsStream(cmd *cobra.Command, args []string) error {
	if _, err := buildEventsStreamFilter(cmd, nil); err != nil {
		return err
	}

	cfg, token, cookie, role, _, err := loadConfigForEvents()
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

	filter, err := buildEventsStreamFilter(cmd, cmdCtx.ResolveChannel)
	if err != nil {
		return err
	}

	includeRaw, _ := cmd.Flags().GetBool("raw")
	human, _ := cmd.Flags().GetBool("human")

	normalizer := newEventNormalizer(cmdCtx)
	socketClient := slack.NewSocketModeClient(cmdCtx.AuthToken, cmdCtx.AuthCookie, cmdCtx.Config.AppToken)
	sink, err := newEventsStreamSink(cmd)
	if err != nil {
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
				line, err := formatStreamEventLine(normalized, human)
				if err != nil {
					return err
				}
				if err := sink.WriteLine(line); err != nil {
					return fmt.Errorf("write event: %w", err)
				}
			}
		}
	}
}
