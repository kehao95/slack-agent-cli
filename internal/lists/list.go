package lists

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/slack"
)

// Fetcher retrieves Slack List items.
type Fetcher interface {
	ListSlackListItems(context.Context, slack.ListItemsParams) (*slack.ListItemsResponse, error)
	GetSlackListItem(context.Context, slack.ListItemInfoParams) (*slack.ListItemInfoResponse, error)
}

// Service coordinates list item retrieval.
type Service struct {
	fetcher Fetcher
}

// Params describes input for listing Slack List items.
type Params struct {
	List     string
	Limit    int
	Cursor   string
	Archived bool
	All      bool
}

// Result represents list item output.
type Result struct {
	ListID     string                   `json:"list_id"`
	Items      []map[string]interface{} `json:"items"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

// ItemParams describes input for fetching one Slack List item.
type ItemParams struct {
	List                string
	ID                  string
	IncludeIsSubscribed bool
}

// ItemResult represents a single list item plus its list metadata.
type ItemResult struct {
	List     map[string]interface{}   `json:"list"`
	Record   map[string]interface{}   `json:"record"`
	Subtasks []map[string]interface{} `json:"subtasks,omitempty"`
}

// NewService constructs a Service.
func NewService(fetcher Fetcher) *Service {
	return &Service{fetcher: fetcher}
}

// ListItems resolves the list reference and fetches items from Slack.
func (s *Service) ListItems(ctx context.Context, params Params) (Result, error) {
	listID, err := ResolveListID(params.List)
	if err != nil {
		return Result{}, err
	}

	if !params.All {
		resp, err := s.fetcher.ListSlackListItems(ctx, slack.ListItemsParams{
			ListID:   listID,
			Limit:    params.Limit,
			Cursor:   params.Cursor,
			Archived: params.Archived,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{
			ListID:     listID,
			Items:      resp.Items,
			NextCursor: resp.ResponseMetadata.NextCursor,
		}, nil
	}

	cursor := params.Cursor
	allItems := make([]map[string]interface{}, 0)
	for {
		resp, err := s.fetcher.ListSlackListItems(ctx, slack.ListItemsParams{
			ListID:   listID,
			Limit:    params.Limit,
			Cursor:   cursor,
			Archived: params.Archived,
		})
		if err != nil {
			return Result{}, err
		}
		allItems = append(allItems, resp.Items...)
		cursor = strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if cursor == "" {
			break
		}
	}

	return Result{
		ListID: listID,
		Items:  allItems,
	}, nil
}

// GetItem resolves the list reference and fetches one record from Slack.
func (s *Service) GetItem(ctx context.Context, params ItemParams) (ItemResult, error) {
	listID, err := ResolveListID(params.List)
	if err != nil {
		return ItemResult{}, err
	}
	resp, err := s.fetcher.GetSlackListItem(ctx, slack.ListItemInfoParams{
		ListID:              listID,
		ID:                  strings.TrimSpace(params.ID),
		IncludeIsSubscribed: params.IncludeIsSubscribed,
	})
	if err != nil {
		return ItemResult{}, err
	}
	return ItemResult{
		List:     resp.List,
		Record:   resp.Record,
		Subtasks: resp.Subtasks,
	}, nil
}

// Lines implements human-readable output for list items.
func (r Result) Lines() []string {
	title := fmt.Sprintf("List %s - %d items", r.ListID, len(r.Items))
	lines := []string{title, strings.Repeat("-", len(title))}
	if len(r.Items) == 0 {
		lines = append(lines, "No items found.")
		return lines
	}
	for _, item := range r.Items {
		itemID, _ := item["id"].(string)
		summary := summarizeRecord(item, nil)
		if itemID == "" {
			lines = append(lines, summary)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", itemID, summary))
	}
	if r.NextCursor != "" {
		lines = append(lines, fmt.Sprintf("Next cursor: %s", r.NextCursor))
	}
	return lines
}

// Lines implements human-readable output for a single list item.
func (r ItemResult) Lines() []string {
	recordID, _ := r.Record["id"].(string)
	listTitle := stringValue(r.List["title"])
	if listTitle == "" {
		listTitle = stringValue(r.List["name"])
	}
	if listTitle == "" {
		listTitle = stringValue(r.List["id"])
	}

	title := fmt.Sprintf("List Item %s", recordID)
	lines := []string{title, strings.Repeat("-", len(title))}
	lines = append(lines, fmt.Sprintf("List: %s", listTitle))
	if listID := stringValue(r.List["id"]); listID != "" {
		lines = append(lines, fmt.Sprintf("List ID: %s", listID))
	}
	if permalink := stringValue(r.List["permalink"]); permalink != "" {
		lines = append(lines, fmt.Sprintf("List URL: %s", permalink))
	}
	if created := formatUnixValue(r.Record["date_created"]); created != "" {
		lines = append(lines, fmt.Sprintf("Created: %s", created))
	}
	if updated := formatUnixLikeValue(r.Record["updated_timestamp"]); updated != "" {
		lines = append(lines, fmt.Sprintf("Updated: %s", updated))
	}
	if subscribed, ok := r.Record["is_subscribed"].(bool); ok {
		lines = append(lines, fmt.Sprintf("Subscribed: %t", subscribed))
	}

	lines = append(lines, "", "Fields:")
	fieldLines := fieldDetailLines(r.Record, schemaByColumnID(r.List))
	if len(fieldLines) == 0 {
		lines = append(lines, "(none)")
	} else {
		lines = append(lines, fieldLines...)
	}
	if len(r.Subtasks) > 0 {
		lines = append(lines, "", fmt.Sprintf("Subtasks: %d", len(r.Subtasks)))
	}
	return lines
}

// ResolveListID accepts a raw Slack List ID or a Slack List URL.
func ResolveListID(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("list is required")
	}
	if isListID(trimmed) {
		return trimmed, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse list reference: %w", err)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for idx := 0; idx < len(parts); idx++ {
		if parts[idx] != "lists" {
			continue
		}
		if idx+2 < len(parts) && isListID(parts[idx+2]) {
			return parts[idx+2], nil
		}
	}
	return "", fmt.Errorf("list not found in reference %q", input)
}

func isListID(input string) bool {
	if !strings.HasPrefix(input, "F") || len(input) < 2 {
		return false
	}
	for _, r := range input[1:] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func summarizeRecord(record map[string]interface{}, schema map[string]schemaColumn) string {
	fieldLines := fieldDetailLines(record, schema)
	if len(fieldLines) == 0 {
		return "item"
	}
	if len(fieldLines) == 1 {
		return trimFieldPrefix(fieldLines[0])
	}
	return trimFieldPrefix(fieldLines[0]) + " | " + trimFieldPrefix(fieldLines[1])
}

type schemaColumn struct {
	ID      string
	Name    string
	Key     string
	Type    string
	Options map[string]string
}

func schemaByColumnID(list map[string]interface{}) map[string]schemaColumn {
	rawMeta, ok := list["list_metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawSchema, ok := rawMeta["schema"].([]interface{})
	if !ok {
		return nil
	}

	result := make(map[string]schemaColumn, len(rawSchema))
	for _, rawCol := range rawSchema {
		colMap, ok := rawCol.(map[string]interface{})
		if !ok {
			continue
		}
		column := schemaColumn{
			ID:      stringValue(colMap["id"]),
			Name:    stringValue(colMap["name"]),
			Key:     stringValue(colMap["key"]),
			Type:    stringValue(colMap["type"]),
			Options: make(map[string]string),
		}
		if options, ok := colMap["options"].(map[string]interface{}); ok {
			if choices, ok := options["choices"].([]interface{}); ok {
				for _, rawChoice := range choices {
					choice, ok := rawChoice.(map[string]interface{})
					if !ok {
						continue
					}
					value := stringValue(choice["value"])
					label := stringValue(choice["label"])
					if value != "" && label != "" {
						column.Options[value] = label
					}
				}
			}
		}
		if column.ID != "" {
			result[column.ID] = column
		}
	}
	return result
}

func fieldDetailLines(record map[string]interface{}, schema map[string]schemaColumn) []string {
	fields, ok := record["fields"].([]interface{})
	if !ok || len(fields) == 0 {
		return nil
	}
	lines := make([]string, 0, len(fields))
	for _, rawField := range fields {
		field, ok := rawField.(map[string]interface{})
		if !ok {
			continue
		}
		label := fieldLabel(field, schema)
		value := formatFieldValue(field, schema)
		if value == "" {
			value = "(empty)"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", label, value))
	}
	return lines
}

func fieldLabel(field map[string]interface{}, schema map[string]schemaColumn) string {
	columnID := stringValue(field["column_id"])
	if columnID != "" && schema != nil {
		if col, ok := schema[columnID]; ok {
			if col.Name != "" {
				return col.Name
			}
			if col.Key != "" {
				return col.Key
			}
		}
	}
	if key := stringValue(field["key"]); key != "" {
		return key
	}
	if columnID != "" {
		return columnID
	}
	return "field"
}

func formatFieldValue(field map[string]interface{}, schema map[string]schemaColumn) string {
	if text := strings.TrimSpace(stringValue(field["text"])); text != "" {
		return text
	}

	column := schemaColumn{}
	if columnID := stringValue(field["column_id"]); columnID != "" && schema != nil {
		column = schema[columnID]
	}

	switch {
	case hasSliceField(field, "select"):
		values := stringSlice(field["select"])
		if len(values) == 0 {
			break
		}
		for i, value := range values {
			if label, ok := column.Options[value]; ok {
				values[i] = label
			}
		}
		return strings.Join(values, ", ")
	case hasSliceField(field, "user"):
		return strings.Join(stringSlice(field["user"]), ", ")
	case hasSliceField(field, "channel"):
		return strings.Join(stringSlice(field["channel"]), ", ")
	case hasSliceField(field, "date"):
		return strings.Join(stringSlice(field["date"]), ", ")
	case hasSliceField(field, "email"):
		return strings.Join(stringSlice(field["email"]), ", ")
	case hasSliceField(field, "phone"):
		return strings.Join(stringSlice(field["phone"]), ", ")
	case hasSliceField(field, "attachment"):
		return strings.Join(stringSlice(field["attachment"]), ", ")
	case hasSliceField(field, "checkbox"):
		bools := boolSlice(field["checkbox"])
		if len(bools) > 0 {
			return strconv.FormatBool(bools[0])
		}
	case hasSliceField(field, "number"):
		return strings.Join(numberSlice(field["number"]), ", ")
	case hasSliceField(field, "rating"):
		numbers := numberSlice(field["rating"])
		if len(numbers) > 0 {
			return numbers[0]
		}
	case hasSliceField(field, "timestamp"):
		values := numberSlice(field["timestamp"])
		if len(values) > 0 {
			return strings.Join(values, ", ")
		}
	case hasSliceField(field, "message"):
		return formatMessageValues(field["message"])
	case hasSliceField(field, "link"):
		return formatLinkValues(field["link"])
	case hasSliceField(field, "reference"):
		return formatReferenceValues(field["reference"])
	}

	switch value := field["value"].(type) {
	case string:
		return strings.TrimSpace(value)
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

func hasSliceField(field map[string]interface{}, key string) bool {
	_, ok := field[key].([]interface{})
	return ok
}

func formatMessageValues(raw interface{}) string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	values := make([]string, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		if value := stringValue(item["value"]); value != "" {
			values = append(values, value)
			continue
		}
		channelID := stringValue(item["channel_id"])
		ts := stringValue(item["ts"])
		if channelID != "" && ts != "" {
			values = append(values, channelID+":"+ts)
		}
	}
	return strings.Join(values, ", ")
}

func formatLinkValues(raw interface{}) string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	values := make([]string, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		displayName := stringValue(item["displayName"])
		originalURL := stringValue(item["originalUrl"])
		switch {
		case displayName != "" && originalURL != "":
			values = append(values, displayName+" ("+originalURL+")")
		case originalURL != "":
			values = append(values, originalURL)
		}
	}
	return strings.Join(values, ", ")
}

func formatReferenceValues(raw interface{}) string {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return ""
	}
	values := make([]string, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		if message, ok := item["message"].(map[string]interface{}); ok {
			channelID := stringValue(message["channel_id"])
			ts := stringValue(message["ts"])
			if channelID != "" && ts != "" {
				values = append(values, "message:"+channelID+":"+ts)
				continue
			}
		}
		if listRecord, ok := item["list_record"].(map[string]interface{}); ok {
			listID := stringValue(listRecord["list_id"])
			rowID := stringValue(listRecord["row_id"])
			if listID != "" && rowID != "" {
				values = append(values, "record:"+listID+":"+rowID)
				continue
			}
		}
		if file, ok := item["file"].(map[string]interface{}); ok {
			if fileID := stringValue(file["file_id"]); fileID != "" {
				values = append(values, "file:"+fileID)
				continue
			}
		}
		if canvas, ok := item["canvas_section"].(map[string]interface{}); ok {
			canvasID := stringValue(canvas["canvas_id"])
			sectionID := stringValue(canvas["section_id"])
			if canvasID != "" || sectionID != "" {
				values = append(values, "canvas:"+canvasID+":"+sectionID)
				continue
			}
		}
	}
	return strings.Join(values, ", ")
}

func stringSlice(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(stringValue(item)); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func boolSlice(raw interface{}) []bool {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	values := make([]bool, 0, len(items))
	for _, item := range items {
		if value, ok := item.(bool); ok {
			values = append(values, value)
		}
	}
	return values
}

func numberSlice(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case float64:
			values = append(values, strconv.FormatFloat(value, 'f', -1, 64))
		case int:
			values = append(values, strconv.Itoa(value))
		case int64:
			values = append(values, strconv.FormatInt(value, 10))
		}
	}
	return values
}

func formatUnixValue(raw interface{}) string {
	switch value := raw.(type) {
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}

func formatUnixLikeValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return formatUnixValue(raw)
	}
}

func stringValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	default:
		return ""
	}
}

func trimFieldPrefix(line string) string {
	trimmed := strings.TrimPrefix(line, "- ")
	if strings.HasPrefix(trimmed, "field: ") {
		return strings.TrimPrefix(trimmed, "field: ")
	}
	return trimmed
}
