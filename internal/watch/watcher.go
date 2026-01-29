package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Config contains watcher configuration options.
type Config struct {
	Channels       []string      // Channel IDs to watch (empty = all)
	IncludeBots    bool          // Include bot messages
	IncludeOwn     bool          // Include own messages
	IncludeThreads bool          // Include thread replies
	Events         []string      // Event types to watch
	JSONOutput     bool          // Output JSON lines
	Quiet          bool          // Suppress connection messages
	Timeout        time.Duration // Exit after timeout (0 = run forever)
	BotUserID      string        // Bot user ID for filtering own messages
}

// Event represents a watch event output.
type Event struct {
	Type        string `json:"type"`
	Timestamp   string `json:"ts"`
	Channel     string `json:"channel"`
	ChannelName string `json:"channel_name,omitempty"`
	User        string `json:"user,omitempty"`
	Username    string `json:"username,omitempty"`
	Text        string `json:"text,omitempty"`
	ThreadTS    string `json:"thread_ts,omitempty"`
	Reaction    string `json:"reaction,omitempty"`
	ItemTS      string `json:"item_ts,omitempty"`
}

// Watcher watches for real-time Slack events via Socket Mode.
type Watcher struct {
	client       *socketmode.Client
	api          *slackapi.Client
	config       Config
	channelCache map[string]string // channel ID -> name
	userCache    map[string]string // user ID -> username
}

// New creates a new Watcher with the provided tokens and configuration.
func New(botToken, appToken string, config Config) *Watcher {
	api := slackapi.New(
		botToken,
		slackapi.OptionAppLevelToken(appToken),
	)
	client := socketmode.New(api)

	return &Watcher{
		client:       client,
		api:          api,
		config:       config,
		channelCache: make(map[string]string),
		userCache:    make(map[string]string),
	}
}

// Run starts the watcher event loop.
func (w *Watcher) Run(ctx context.Context) error {
	// Set up timeout if configured
	if w.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.config.Timeout)
		defer cancel()
	}

	// Pre-populate caches if watching specific channels
	if err := w.initializeCaches(ctx); err != nil {
		return fmt.Errorf("initialize caches: %w", err)
	}

	if !w.config.Quiet {
		w.logStatus("Connecting to Slack...")
	}

	// Start event loop in goroutine
	go w.eventLoop(ctx)

	// Run the socket mode client (this blocks until context is cancelled)
	if err := w.client.RunContext(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("socket mode error: %w", err)
	}

	return nil
}

func (w *Watcher) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-w.client.Events:
			w.handleEvent(ctx, evt)
		}
	}
}

func (w *Watcher) handleEvent(ctx context.Context, evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeConnecting:
		if !w.config.Quiet {
			w.logStatus("Connecting...")
		}

	case socketmode.EventTypeConnectionError:
		if !w.config.Quiet {
			w.logStatus("Connection error")
		}

	case socketmode.EventTypeConnected:
		if !w.config.Quiet {
			w.logStatus("Connected. Watching for events...")
		}

	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}

		// Acknowledge the event
		if w.client != nil {
			w.client.Ack(*evt.Request)
		}

		w.handleInnerEvent(ctx, eventsAPIEvent.InnerEvent)

	case socketmode.EventTypeHello:
		// Initial connection handshake - no action needed
	}
}

func (w *Watcher) handleInnerEvent(ctx context.Context, evt slackevents.EventsAPIInnerEvent) {
	// Check if this event type is enabled
	if !w.isEventEnabled(evt.Type) {
		return
	}

	switch evt.Type {
	case "message":
		if msgEvt, ok := evt.Data.(*slackapi.MessageEvent); ok {
			w.handleMessage(ctx, msgEvt)
		}

	case "reaction_added":
		if rxnEvt, ok := evt.Data.(*slackapi.ReactionAddedEvent); ok {
			w.handleReaction(ctx, "reaction_added", rxnEvt)
		}

	case "reaction_removed":
		if rxnEvt, ok := evt.Data.(*slackapi.ReactionRemovedEvent); ok {
			w.handleReactionRemoved(ctx, rxnEvt)
		}
	}
}

func (w *Watcher) handleMessage(ctx context.Context, msg *slackapi.MessageEvent) {
	// Filter by channel if specified
	if !w.shouldWatchChannel(msg.Channel) {
		return
	}

	// Filter bot messages
	if msg.BotID != "" && !w.config.IncludeBots {
		return
	}

	// Filter own messages
	if msg.User == w.config.BotUserID && !w.config.IncludeOwn {
		return
	}

	// Filter thread replies
	if msg.ThreadTimestamp != "" && msg.ThreadTimestamp != msg.Timestamp && !w.config.IncludeThreads {
		return
	}

	// Skip message subtypes we don't care about (like channel_join, etc.)
	if msg.SubType != "" && msg.SubType != "thread_broadcast" {
		return
	}

	event := Event{
		Type:      "message",
		Timestamp: msg.Timestamp,
		Channel:   msg.Channel,
		User:      msg.User,
		Text:      msg.Text,
	}

	if msg.ThreadTimestamp != "" && msg.ThreadTimestamp != msg.Timestamp {
		event.ThreadTS = msg.ThreadTimestamp
	}

	w.enrichAndOutput(ctx, event)
}

func (w *Watcher) handleReaction(ctx context.Context, eventType string, rxn *slackapi.ReactionAddedEvent) {
	// Filter by channel if specified
	if !w.shouldWatchChannel(rxn.Item.Channel) {
		return
	}

	// Filter own reactions
	if rxn.User == w.config.BotUserID && !w.config.IncludeOwn {
		return
	}

	event := Event{
		Type:      eventType,
		Timestamp: rxn.Item.Timestamp,
		Channel:   rxn.Item.Channel,
		User:      rxn.User,
		Reaction:  rxn.Reaction,
		ItemTS:    rxn.Item.Timestamp,
	}

	w.enrichAndOutput(ctx, event)
}

func (w *Watcher) handleReactionRemoved(ctx context.Context, rxn *slackapi.ReactionRemovedEvent) {
	// Filter by channel if specified
	if !w.shouldWatchChannel(rxn.Item.Channel) {
		return
	}

	// Filter own reactions
	if rxn.User == w.config.BotUserID && !w.config.IncludeOwn {
		return
	}

	event := Event{
		Type:      "reaction_removed",
		Timestamp: rxn.Item.Timestamp,
		Channel:   rxn.Item.Channel,
		User:      rxn.User,
		Reaction:  rxn.Reaction,
		ItemTS:    rxn.Item.Timestamp,
	}

	w.enrichAndOutput(ctx, event)
}

func (w *Watcher) enrichAndOutput(ctx context.Context, event Event) {
	// Resolve channel name
	if name, ok := w.channelCache[event.Channel]; ok {
		event.ChannelName = name
	} else {
		// Try to fetch channel info
		if info, err := w.api.GetConversationInfoContext(ctx, &slackapi.GetConversationInfoInput{
			ChannelID: event.Channel,
		}); err == nil {
			w.channelCache[event.Channel] = info.Name
			event.ChannelName = info.Name
		}
	}

	// Resolve username
	if event.User != "" {
		if name, ok := w.userCache[event.User]; ok {
			event.Username = name
		} else {
			// Try to fetch user info
			if user, err := w.api.GetUserInfoContext(ctx, event.User); err == nil {
				w.userCache[event.User] = user.Name
				event.Username = user.Name
			}
		}
	}

	w.outputEvent(event)
}

func (w *Watcher) outputEvent(event Event) {
	if w.config.JSONOutput {
		data, _ := json.Marshal(event)
		fmt.Println(string(data))
	} else {
		fmt.Println(w.formatHumanReadable(event))
	}
}

func (w *Watcher) formatHumanReadable(event Event) string {
	timestamp := w.formatTimestamp(event.Timestamp)
	channelDisplay := fmt.Sprintf("#%s", event.ChannelName)
	if event.ChannelName == "" {
		channelDisplay = event.Channel
	}

	userDisplay := fmt.Sprintf("@%s", event.Username)
	if event.Username == "" {
		userDisplay = event.User
	}

	switch event.Type {
	case "message":
		threadMarker := ""
		if event.ThreadTS != "" {
			threadMarker = " (thread)"
		}
		return fmt.Sprintf("[%s] %s | %s%s: %s", timestamp, channelDisplay, userDisplay, threadMarker, event.Text)

	case "reaction_added":
		return fmt.Sprintf("[%s] %s | %s reacted with :%s:", timestamp, channelDisplay, userDisplay, event.Reaction)

	case "reaction_removed":
		return fmt.Sprintf("[%s] %s | %s removed reaction :%s:", timestamp, channelDisplay, userDisplay, event.Reaction)

	default:
		return fmt.Sprintf("[%s] %s | %s: %s", timestamp, channelDisplay, userDisplay, event.Type)
	}
}

func (w *Watcher) formatTimestamp(ts string) string {
	// Parse Slack timestamp (format: "1234567890.123456")
	parts := strings.Split(ts, ".")
	if len(parts) == 0 {
		return ts
	}

	var unixTime int64
	fmt.Sscanf(parts[0], "%d", &unixTime)
	t := time.Unix(unixTime, 0)
	return t.Format("2006-01-02 15:04:05")
}

func (w *Watcher) logStatus(msg string) {
	if w.config.JSONOutput {
		// Don't log status in JSON mode
		return
	}
	fmt.Printf("# %s\n", msg)
}

func (w *Watcher) isEventEnabled(eventType string) bool {
	if len(w.config.Events) == 0 {
		return true
	}
	for _, et := range w.config.Events {
		if et == eventType {
			return true
		}
	}
	return false
}

func (w *Watcher) shouldWatchChannel(channelID string) bool {
	if len(w.config.Channels) == 0 {
		return true // Watch all channels
	}
	for _, ch := range w.config.Channels {
		if ch == channelID {
			return true
		}
	}
	return false
}

func (w *Watcher) initializeCaches(ctx context.Context) error {
	// Pre-populate channel cache for specific channels
	if len(w.config.Channels) > 0 {
		for _, channelID := range w.config.Channels {
			info, err := w.api.GetConversationInfoContext(ctx, &slackapi.GetConversationInfoInput{
				ChannelID: channelID,
			})
			if err != nil {
				// Don't fail hard - just skip this channel
				continue
			}
			w.channelCache[channelID] = info.Name
		}
	}

	return nil
}
