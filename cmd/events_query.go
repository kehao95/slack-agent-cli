package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/config"
	cerrors "github.com/kehao95/slack-agent-cli/internal/errors"
	"github.com/kehao95/slack-agent-cli/internal/eventstore"
	"github.com/spf13/cobra"
)

var eventsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached events",
	Long:  "Query the local daemon-backed event cache and return matching retained events.",
	Example: `  # Get all retained events
  slk events list

  # Get recent channel events
  slk events list --channel "#_bot-testing" --since 1h

  # Get cached thread activity
  slk events list --thread "1776957488.977069" --exclude-self`,
	RunE: runEventsList,
}

var eventsGetCmd = &cobra.Command{
	Use:    "get",
	Short:  "Alias for events list",
	Hidden: true,
	RunE:   runEventsList,
}

var eventsNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Wait for the next matching cached event",
	Long: `Return the first cached event after --since that matches the filters.

If no matching event is currently cached, the command blocks until one arrives or --timeout expires.`,
	Example: `  # Wait for new activity after a cursor
  slk events next --since 123 --timeout 60s

  # Wait for the next reply in a thread
  slk events next --thread "1776957488.977069" --since 123 --timeout 5m`,
	RunE: runEventsNext,
}

var eventsClaimCmd = &cobra.Command{
	Use:   "claim",
	Short: "Claim the oldest eligible cached event for processing",
	Long: `Atomically claim one eligible event from the local queue and return it.

The command blocks until a claimable event exists or the process is interrupted.
Claimed events are removed from the "pending" set until acked or the lease expires.
Use the returned cursor field with "slk events ack <cursor>" after successful processing.
If processing fails, do not ack; the event becomes claimable again after --lease.`,
	Example: `  # Claim one message event and block until work is available
  slk events claim --type message --lease 5m

  # Claim only messages that mention the active auth user
  slk events claim --type message --mentions-me

  # Claim only top-level channel messages for task assignment
  slk events claim --type message --message-kind root --channel "#_bot-testing"

  # Claim only replies inside a specific thread
  slk events claim --type message --message-kind reply --thread "1776996811.296809" --lease 10m`,
	RunE: runEventsClaim,
}

var eventsAckCmd = &cobra.Command{
	Use:   "ack <cursor>",
	Short: "Acknowledge a claimed event as processed",
	Long: `Mark a previously claimed event cursor as acknowledged.

Use this in your agent loop after successful processing.
Re-claiming before ack within the lease window is prevented.`,
	Args: cobra.ExactArgs(1),
	Example: `  # Mark a claimed event as done
  slk events ack 123`,
	RunE: runEventsAck,
}

func init() {
	eventsCmd.AddCommand(eventsGetCmd)
	eventsCmd.AddCommand(eventsListCmd)
	eventsCmd.AddCommand(eventsNextCmd)
	eventsCmd.AddCommand(eventsClaimCmd)
	eventsCmd.AddCommand(eventsAckCmd)

	addEventQueryFlags(eventsListCmd, false)
	addEventQueryFlags(eventsGetCmd, false)
	addEventQueryFlags(eventsNextCmd, true)
	addEventClaimFlags(eventsClaimCmd)
	eventsClaimCmd.Flags().Duration("lease", 5*time.Minute, "Lease duration for the claimed event")
}

func addEventQueryFlags(cmd *cobra.Command, includeTimeout bool) {
	cmd.Flags().String("channel", "", "Restrict to a single channel/conversation name or ID")
	cmd.Flags().String("type", "", "Restrict to event type, for example message")
	cmd.Flags().String("conversation-type", "", "Filter by conversation types: channel,private,dm,mpdm,app_home")
	cmd.Flags().String("thread", "", "Restrict to a specific thread_ts")
	cmd.Flags().String("user", "", "Restrict to a Slack user ID")
	cmd.Flags().String("since", "", "Start after local cursor, or from duration/RFC3339 time (next defaults to latest)")
	cmd.Flags().Int("limit", 100, "Maximum events to return")
	cmd.Flags().Bool("threads-only", false, "Only return thread-related message events")
	cmd.Flags().Bool("exclude-self", false, "Exclude events produced by the active auth identity")
	if includeTimeout {
		cmd.Flags().Duration("timeout", 0, "Maximum time to wait for a matching event (0 waits forever)")
	}
}

func addEventClaimFlags(cmd *cobra.Command) {
	cmd.Flags().String("channel", "", "Restrict to a single channel/conversation name or ID")
	cmd.Flags().String("type", "", "Restrict to event type, for example message")
	cmd.Flags().String("message-kind", "", "Restrict message events to root, reply, or thread")
	cmd.Flags().Bool("mentions-me", false, "Only return message events that contain a Slack mention of the active auth user")
	cmd.Flags().String("conversation-type", "", "Filter by conversation types: channel,private,dm,mpdm,app_home")
	cmd.Flags().String("thread", "", "Restrict to a specific thread_ts")
	cmd.Flags().String("user", "", "Restrict to a Slack user ID")
	cmd.Flags().Bool("exclude-self", false, "Exclude events produced by the active auth identity")
}

func runEventsList(cmd *cobra.Command, args []string) error {
	cmdCtx, _, store, err := openEventQueryStore(cmd, false)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	defer store.Close()

	filter, err := buildEventQueryFilter(cmd, cmdCtx, store, false)
	if err != nil {
		return err
	}
	events, err := store.Query(cmdCtx.Ctx, filter)
	if err != nil {
		return err
	}
	return printCachedEvents(cmd, events)
}

func runEventsNext(cmd *cobra.Command, args []string) error {
	cmdCtx, _, store, err := openEventQueryStore(cmd, true)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	defer store.Close()

	filter, err := buildEventQueryFilter(cmd, cmdCtx, store, true)
	if err != nil {
		return err
	}
	filter.Limit = 1

	timeout, _ := cmd.Flags().GetDuration("timeout")
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, err := store.Query(cmdCtx.Ctx, filter)
		if err != nil {
			return err
		}
		if len(events) > 0 {
			return printCachedEvent(cmd, events[0])
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			cmd.SilenceUsage = true
			return cerrors.TimeoutError("timed out waiting for matching event")
		}
		select {
		case <-cmdCtx.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func runEventsClaim(cmd *cobra.Command, args []string) error {
	cmdCtx, _, store, err := openEventQueryStore(cmd, true)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	defer store.Close()

	filter, err := buildEventClaimFilter(cmd, cmdCtx)
	if err != nil {
		return err
	}
	filter.Limit = 1

	lease, _ := cmd.Flags().GetDuration("lease")
	if lease <= 0 {
		return fmt.Errorf("invalid --lease %q, must be greater than zero", lease)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		claimed, ok, err := store.Claim(cmdCtx.Ctx, filter, lease)
		if err != nil {
			return err
		}
		if ok {
			return printClaimedEvent(cmd, claimed)
		}
		select {
		case <-cmdCtx.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func buildEventClaimFilter(cmd *cobra.Command, cmdCtx *CommandContext) (eventstore.Filter, error) {
	channelInput, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if strings.TrimSpace(channelInput) != "" {
		resolved, err := cmdCtx.ResolveChannel(channelInput)
		if err != nil {
			return eventstore.Filter{}, err
		}
		channelID = resolved
	}

	conversationTypeArg, _ := cmd.Flags().GetString("conversation-type")
	conversationTypes, err := parseConversationTypes(conversationTypeArg)
	if err != nil {
		return eventstore.Filter{}, err
	}

	threadTS, _ := cmd.Flags().GetString("thread")
	userID, _ := cmd.Flags().GetString("user")
	eventType, _ := cmd.Flags().GetString("type")
	messageKind, err := parseMessageKindFlag(cmd)
	if err != nil {
		return eventstore.Filter{}, err
	}
	excludeSelf, _ := cmd.Flags().GetBool("exclude-self")
	mentionsMe, _ := cmd.Flags().GetBool("mentions-me")
	mentionUserID := ""
	if mentionsMe {
		if err := cmdCtx.EnsureAuthIdentity(cmdCtx.Ctx); err != nil {
			return eventstore.Filter{}, err
		}
		mentionUserID = strings.TrimSpace(cmdCtx.AuthUserID)
		if mentionUserID == "" {
			return eventstore.Filter{}, fmt.Errorf("--mentions-me requires an authenticated Slack user id")
		}
	}

	return eventstore.Filter{
		ChannelID:         channelID,
		Type:              strings.TrimSpace(eventType),
		MessageKind:       messageKind,
		MentionUserID:     mentionUserID,
		ConversationTypes: conversationTypes,
		ThreadTS:          strings.TrimSpace(threadTS),
		UserID:            strings.TrimSpace(userID),
		ExcludeSelf:       excludeSelf,
		Limit:             1,
	}, nil
}

func parseMessageKindFlag(cmd *cobra.Command) (string, error) {
	raw, _ := cmd.Flags().GetString("message-kind")
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", "root", "reply", "thread":
		return raw, nil
	default:
		return "", fmt.Errorf("invalid --message-kind %q: use root, reply, or thread", raw)
	}
}

func runEventsAck(cmd *cobra.Command, args []string) error {
	cursor, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid cursor %q: must be a positive integer", args[0])
	}
	if cursor <= 0 {
		return fmt.Errorf("cursor must be positive")
	}

	cmdCtx, _, store, err := openEventQueryStore(cmd, false)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	defer store.Close()

	acked, err := store.Ack(cmdCtx.Ctx, cursor)
	if err != nil {
		return err
	}

	human, _ := cmd.Flags().GetBool("human")
	if human {
		_, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"ack cursor=%d ok=%t already_acked=%t\n",
			cursor,
			acked,
			!acked,
		)
		return err
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(map[string]interface{}{
		"ok":            true,
		"cursor":        cursor,
		"acked":         acked,
		"already_acked": !acked,
	})
}

func openEventQueryStore(cmd *cobra.Command, streaming bool) (*CommandContext, string, *eventstore.Store, error) {
	_, configPath, err := config.Load(cfgFile)
	if err != nil {
		return nil, "", nil, cerrors.ConfigError("failed to load config: %w", err)
	}
	var cmdCtx *CommandContext
	if streaming {
		cmdCtx, err = NewStreamingCommandContext(cmd)
	} else {
		cmdCtx, err = NewCommandContext(cmd, 0)
	}
	if err != nil {
		return nil, "", nil, err
	}
	dbPath, err := eventstore.DefaultPath(configPath, cmdCtx.TeamID)
	if err != nil {
		cmdCtx.Close()
		return nil, "", nil, cerrors.ConfigError("resolve event store path: %w", err)
	}
	store, err := eventstore.Open(dbPath)
	if err != nil {
		cmdCtx.Close()
		return nil, "", nil, fmt.Errorf("open event store: %w", err)
	}
	return cmdCtx, configPath, store, nil
}

func buildEventQueryFilter(cmd *cobra.Command, cmdCtx *CommandContext, store *eventstore.Store, nextMode bool) (eventstore.Filter, error) {
	channelInput, _ := cmd.Flags().GetString("channel")
	channelID := ""
	if strings.TrimSpace(channelInput) != "" {
		resolved, err := cmdCtx.ResolveChannel(channelInput)
		if err != nil {
			return eventstore.Filter{}, err
		}
		channelID = resolved
	}

	conversationTypeArg, _ := cmd.Flags().GetString("conversation-type")
	conversationTypes, err := parseConversationTypes(conversationTypeArg)
	if err != nil {
		return eventstore.Filter{}, err
	}

	threadTS, _ := cmd.Flags().GetString("thread")
	userID, _ := cmd.Flags().GetString("user")
	since, _ := cmd.Flags().GetString("since")
	eventType, _ := cmd.Flags().GetString("type")
	limit, _ := cmd.Flags().GetInt("limit")
	threadsOnly, _ := cmd.Flags().GetBool("threads-only")
	excludeSelf, _ := cmd.Flags().GetBool("exclude-self")

	filter := eventstore.Filter{
		ChannelID:         channelID,
		Type:              strings.TrimSpace(eventType),
		ConversationTypes: conversationTypes,
		ThreadTS:          strings.TrimSpace(threadTS),
		UserID:            strings.TrimSpace(userID),
		ThreadsOnly:       threadsOnly,
		ExcludeSelf:       excludeSelf,
		Limit:             limit,
	}
	if strings.TrimSpace(since) == "" && nextMode {
		cursor, err := store.LatestCursor(cmdCtx.Ctx)
		if err != nil {
			return eventstore.Filter{}, err
		}
		filter.SinceCursor = cursor
		return filter, nil
	}
	if err := applySince(cmdCtx.Ctx, store, &filter, since); err != nil {
		return eventstore.Filter{}, err
	}
	return filter, nil
}

func applySince(ctx context.Context, store *eventstore.Store, filter *eventstore.Filter, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if raw == "latest" {
		cursor, err := store.LatestCursor(ctx)
		if err != nil {
			return err
		}
		filter.SinceCursor = cursor
		return nil
	}
	if cursor, err := strconv.ParseInt(raw, 10, 64); err == nil {
		filter.SinceCursor = cursor
		return nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		filter.SinceReceivedAt = time.Now().Add(-d)
		return nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		filter.SinceReceivedAt = ts
		return nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		filter.SinceReceivedAt = ts
		return nil
	}
	return fmt.Errorf("invalid --since %q: use local cursor, latest, duration like 1h, or RFC3339 time", raw)
}

func printCachedEvents(cmd *cobra.Command, events []eventstore.Event) error {
	human, _ := cmd.Flags().GetBool("human")
	if human {
		for _, event := range events {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatHumanStreamEvent(streamEventFromStore(event))); err != nil {
				return fmt.Errorf("print event: %w", err)
			}
		}
		return nil
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(events)
}

func printCachedEvent(cmd *cobra.Command, event eventstore.Event) error {
	human, _ := cmd.Flags().GetBool("human")
	if human {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), formatHumanStreamEvent(streamEventFromStore(event)))
		return err
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(event)
}

func printClaimedEvent(cmd *cobra.Command, claimed eventstore.ClaimedEvent) error {
	human, _ := cmd.Flags().GetBool("human")
	if human {
		_, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s lease_until=%s\n",
			formatHumanStreamEvent(streamEventFromStore(claimed.Event)),
			claimed.LeaseUntil.Format(time.RFC3339Nano),
		)
		return err
	}
	payload := struct {
		eventstore.Event
		LeaseUntil time.Time `json:"lease_until"`
		ClaimedAt  time.Time `json:"claimed_at"`
	}{
		Event:      claimed.Event,
		LeaseUntil: claimed.LeaseUntil,
		ClaimedAt:  claimed.ClaimedAt,
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return encoder.Encode(payload)
}
