package cmd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/output"
	"github.com/spf13/cobra"
)

var messagesGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Get one message by URL or channel and timestamp",
	Example: "  slk messages get --message https://workspace.slack.com/archives/C123/p1705312365000100\n  slk messages get --channel C123 --ts 1705312365.000100 --include-thread",
	RunE:    runMessagesGet,
}

func init() {
	messagesCmd.AddCommand(messagesGetCmd)
	messagesGetCmd.Flags().StringP("message", "m", "", "Slack message URL")
	messagesGetCmd.Flags().String("url", "", "Alias for --message")
	messagesGetCmd.Flags().StringP("channel", "c", "", "Channel ID or name")
	messagesGetCmd.Flags().String("ts", "", "Message timestamp")
	messagesGetCmd.Flags().String("thread-ts", "", "Root thread timestamp when --ts identifies a reply")
	messagesGetCmd.Flags().Bool("include-thread", false, "Include the complete thread")
	messagesGetCmd.Flags().Bool("thread", false, "Alias for --include-thread")
}

var slackMessagePathPattern = regexp.MustCompile(`^p([0-9]{11,})$`)

// ParseSlackMessageURL extracts a conversation ID and Slack timestamp from a permalink.
func ParseSlackMessageURL(reference string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || u.Host == "" || u.User != nil || u.Port() != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", fmt.Errorf("invalid Slack message URL")
	}
	host := strings.ToLower(u.Hostname())
	if host != "slack.com" && !strings.HasSuffix(host, ".slack.com") {
		return "", "", fmt.Errorf("message URL must point to Slack")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "archives" || !isChannelID(strings.ToUpper(parts[1])) {
		return "", "", fmt.Errorf("invalid Slack message URL path")
	}
	match := slackMessagePathPattern.FindStringSubmatch(parts[2])
	if len(match) != 2 {
		return "", "", fmt.Errorf("Slack message URL has an invalid timestamp")
	}
	digits := match[1]
	if len(digits) < 7 {
		return "", "", fmt.Errorf("Slack message URL has an invalid timestamp")
	}
	seconds := digits[:len(digits)-6]
	micros := digits[len(digits)-6:]
	return strings.ToUpper(parts[1]), seconds + "." + micros, nil
}

func runMessagesGet(cmd *cobra.Command, _ []string) error {
	messageURL, _ := cmd.Flags().GetString("message")
	if messageURL == "" {
		messageURL, _ = cmd.Flags().GetString("url")
	}
	channelRef, _ := cmd.Flags().GetString("channel")
	timestamp, _ := cmd.Flags().GetString("ts")
	if messageURL != "" {
		if channelRef != "" || timestamp != "" {
			return fmt.Errorf("use --message URL or --channel with --ts, not both")
		}
		var err error
		channelRef, timestamp, err = ParseSlackMessageURL(messageURL)
		if err != nil {
			return err
		}
	} else if strings.TrimSpace(channelRef) == "" || strings.TrimSpace(timestamp) == "" {
		return fmt.Errorf("provide --message URL or both --channel and --ts")
	}
	includeThread, _ := cmd.Flags().GetBool("include-thread")
	threadAlias, _ := cmd.Flags().GetBool("thread")
	includeThread = includeThread || threadAlias
	threadRoot := ""
	if messageURL != "" {
		parsedURL, parseErr := url.Parse(messageURL)
		if parseErr == nil {
			threadRoot = parsedURL.Query().Get("thread_ts")
		}
	}
	if explicitRoot, _ := cmd.Flags().GetString("thread-ts"); explicitRoot != "" {
		if threadRoot != "" && threadRoot != explicitRoot {
			return fmt.Errorf("message URL thread_ts and --thread-ts disagree")
		}
		threadRoot = explicitRoot
	}

	cmdCtx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()
	channelID, err := cmdCtx.ResolveChannel(channelRef)
	if err != nil {
		return err
	}
	result, err := cmdCtx.Client.GetMessage(cmdCtx.Ctx, channelID, timestamp, includeThread, threadRoot)
	if err != nil {
		return err
	}
	result.Channel = channelRef
	return output.Print(cmd, result)
}
