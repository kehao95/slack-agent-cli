package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api <family.method>",
	Short: "Call any Slack Web API method",
	Long: `Call a Slack Web API method using the active user or bot credentials.

The request body must be a JSON object. Pass it inline, read it from a file with
--data @path.json, or read it from stdin with --data -. The response is emitted
as Slack JSON without SDK-specific reshaping.

When --all is set, cursor pagination is followed until response_metadata.next_cursor
is empty. The output becomes an object containing every complete Slack response
under "pages". HTTP 429 responses honor Retry-After and are retried automatically.`,
	Example: `  # Call a method with inline JSON
  slk api conversations.info --data '{"channel":"C123"}'

  # Read a request from a file or stdin
  slk api chat.postMessage --data @request.json
  printf '{"channel":"C123","limit":100}' | slk api conversations.history --data -

  # Fetch every cursor page, starting at an optional cursor
  slk api conversations.list --data '{"limit":200}' --all
  slk api users.list --cursor dXNlcjpVMTIz --all`,
	Args: cobra.ExactArgs(1),
	RunE: runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().String("data", "{}", "JSON object, @file path, or - for stdin")
	apiCmd.Flags().String("cursor", "", "Initial cursor (sets the request cursor field)")
	apiCmd.Flags().Bool("all", false, "Fetch all cursor pages")
	apiCmd.Flags().Int("max-retries", 3, "Maximum retries after Slack HTTP 429 responses")
	apiCmd.Flags().Duration("page-delay", 0, "Delay between pagination requests")
}

type apiPagesOutput struct {
	OK        bool              `json:"ok"`
	PageCount int               `json:"page_count"`
	Pages     []json.RawMessage `json:"pages"`
}

func runAPI(cmd *cobra.Command, args []string) error {
	method := strings.TrimSpace(args[0])
	if err := slack.ValidateAPIMethod(method); err != nil {
		return err
	}

	data, _ := cmd.Flags().GetString("data")
	payload, err := readAPIData(cmd, data)
	if err != nil {
		return err
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	allPages, _ := cmd.Flags().GetBool("all")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	pageDelay, _ := cmd.Flags().GetDuration("page-delay")
	if maxRetries < 0 {
		return fmt.Errorf("max-retries cannot be negative")
	}
	if pageDelay < 0 {
		return fmt.Errorf("page-delay cannot be negative")
	}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		payload["cursor"] = cursor
	}

	cmdCtx, err := NewCommandContext(cmd, 10*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	options := slack.CallAPIOptions{MaxRetries: maxRetries}
	first, err := cmdCtx.Client.CallAPI(cmdCtx.Ctx, method, payload, options)
	if err != nil {
		return fmt.Errorf("call Slack API method %s: %w", method, err)
	}
	if !allPages {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(first))
		return err
	}

	pages := []json.RawMessage{first}
	seen := make(map[string]struct{})
	for cursor = slack.NextCursor(first); cursor != ""; cursor = slack.NextCursor(pages[len(pages)-1]) {
		if _, duplicate := seen[cursor]; duplicate {
			return fmt.Errorf("call Slack API method %s: repeated pagination cursor %q", method, cursor)
		}
		seen[cursor] = struct{}{}
		if pageDelay > 0 {
			timer := time.NewTimer(pageDelay)
			select {
			case <-cmdCtx.Ctx.Done():
				timer.Stop()
				return cmdCtx.Ctx.Err()
			case <-timer.C:
			}
		}
		payload["cursor"] = cursor
		page, callErr := cmdCtx.Client.CallAPI(cmdCtx.Ctx, method, payload, options)
		if callErr != nil {
			return fmt.Errorf("call Slack API method %s: %w", method, callErr)
		}
		pages = append(pages, page)
	}

	encoded, err := json.Marshal(apiPagesOutput{OK: true, PageCount: len(pages), Pages: pages})
	if err != nil {
		return fmt.Errorf("encode paginated API response: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
	return err
}

func readAPIData(cmd *cobra.Command, source string) (map[string]interface{}, error) {
	var (
		data []byte
		err  error
	)
	switch {
	case source == "-":
		data, err = io.ReadAll(cmd.InOrStdin())
	case strings.HasPrefix(source, "@"):
		path := strings.TrimSpace(strings.TrimPrefix(source, "@"))
		if path == "" {
			return nil, fmt.Errorf("data file path is empty")
		}
		data, err = os.ReadFile(path)
	default:
		data = []byte(source)
	}
	if err != nil {
		return nil, fmt.Errorf("read API request data: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("API request data is empty")
	}

	payload := make(map[string]interface{})
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse API request data as a JSON object: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("parse API request data: expected a JSON object")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("parse API request data: expected exactly one JSON object")
	}
	return payload, nil
}
