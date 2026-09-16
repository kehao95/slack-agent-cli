package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kehao95/slack-agent-cli/internal/lists"
	"github.com/kehao95/slack-agent-cli/internal/output"
	appslack "github.com/kehao95/slack-agent-cli/internal/slack"
	"github.com/spf13/cobra"
)

var listsCreateCmd = &cobra.Command{Use: "create", Short: "Create a Slack List", RunE: runListsCreate}
var listsUpdateCmd = &cobra.Command{Use: "update", Short: "Update Slack List metadata", RunE: runListsUpdate}
var listsItemAddCmd = &cobra.Command{Use: "item-add", Short: "Add an item to a Slack List", RunE: runListsItemAdd}
var listsItemUpdateCmd = &cobra.Command{Use: "item-update", Short: "Update cells in a Slack List item", RunE: runListsItemUpdate}
var listsItemDeleteCmd = &cobra.Command{Use: "item-delete", Short: "Delete an item from a Slack List", RunE: runListsItemDelete}
var listsItemsDeleteCmd = &cobra.Command{Use: "items-delete", Short: "Delete multiple Slack List items", RunE: runListsItemsDelete}
var listsShareCmd = &cobra.Command{Use: "share", Short: "Share a Slack List", RunE: runListsShare}
var listsRevokeCmd = &cobra.Command{Use: "revoke", Short: "Revoke Slack List access", RunE: runListsRevoke}
var listsExportCmd = &cobra.Command{Use: "export", Short: "Start or retrieve a Slack List export", RunE: runListsExport}
var listsExportStartCmd = &cobra.Command{Use: "export-start", Short: "Start a Slack List export job", RunE: runListsExportStart}
var listsExportGetCmd = &cobra.Command{Use: "export-get", Short: "Retrieve a Slack List export job", RunE: runListsExportGet}

func init() {
	listsCmd.AddCommand(listsCreateCmd, listsUpdateCmd, listsItemAddCmd, listsItemUpdateCmd, listsItemDeleteCmd, listsItemsDeleteCmd, listsShareCmd, listsRevokeCmd, listsExportCmd, listsExportStartCmd, listsExportGetCmd)

	listsCreateCmd.Flags().String("name", "", "List name (required)")
	listsCreateCmd.Flags().String("description-blocks", "", "Rich text description blocks JSON, @file, or -")
	listsCreateCmd.Flags().String("schema", "", "Column schema JSON array, @file, or -")
	listsCreateCmd.Flags().String("copy-from", "", "List ID to copy")
	listsCreateCmd.Flags().Bool("include-copied-records", false, "Copy records from --copy-from")
	listsCreateCmd.Flags().Bool("todo-mode", false, "Create task tracking fields")

	listsUpdateCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsUpdateCmd.Flags().String("name", "", "Replacement List name")
	listsUpdateCmd.Flags().String("description-blocks", "", "Replacement rich text description blocks JSON, @file, or -")
	listsUpdateCmd.Flags().Bool("todo-mode", false, "Enable or disable task tracking fields")

	listsItemAddCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsItemAddCmd.Flags().String("fields", "", "Initial fields JSON, @file, or -")
	listsItemAddCmd.Flags().String("duplicate", "", "Record ID to duplicate")
	listsItemAddCmd.Flags().String("parent", "", "Parent record ID for a subtask")

	listsItemUpdateCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsItemUpdateCmd.Flags().String("item", "", "Record ID (required)")
	listsItemUpdateCmd.Flags().String("cells", "", "Cells JSON, @file, or - (required)")

	listsItemDeleteCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsItemDeleteCmd.Flags().String("item", "", "Record ID (required)")
	listsItemsDeleteCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsItemsDeleteCmd.Flags().String("items", "", "Comma-separated record IDs (required)")

	for _, command := range []*cobra.Command{listsShareCmd, listsRevokeCmd} {
		command.Flags().String("list", "", "List ID or Slack List URL (required)")
		command.Flags().String("channels", "", "Comma-separated channel names or IDs")
		command.Flags().String("users", "", "Comma-separated user IDs, <@ID> mentions, or @usernames")
	}
	listsShareCmd.Flags().String("access-level", "", "Access level: read, write, or owner (required)")

	for _, command := range []*cobra.Command{listsExportCmd, listsExportStartCmd} {
		command.Flags().String("list", "", "List ID or Slack List URL (required)")
		command.Flags().Bool("include-archived", false, "Include archived records")
		command.Flags().String("format", "csv", "Export format: csv or json")
		command.Flags().Bool("include-threads", false, "Include item threads (JSON only)")
		command.Flags().Bool("include-attachments", false, "Include attachment metadata (JSON only)")
	}
	listsExportCmd.Flags().String("job", "", "Existing export job ID; omit to start a job")
	listsExportCmd.Flags().StringP("output", "o", "", "Download destination for a completed export")
	listsExportCmd.Flags().Bool("force", false, "Overwrite an existing export destination")
	listsExportGetCmd.Flags().String("list", "", "List ID or Slack List URL (required)")
	listsExportGetCmd.Flags().String("job", "", "Export job ID (required)")
	listsExportGetCmd.Flags().String("format", "csv", "Export format: csv or json")
	listsExportGetCmd.Flags().Bool("include-threads", false, "Include item threads (JSON only)")
	listsExportGetCmd.Flags().Bool("include-attachments", false, "Include attachment metadata (JSON only)")
}

func listIDFlag(cmd *cobra.Command) (string, error) {
	ref := artifactFlagString(cmd, "list")
	if ref == "" {
		return "", fmt.Errorf("--list is required")
	}
	return lists.ResolveListID(ref)
}

func parseListJSON(value, flag string) (interface{}, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	resolved, err := readJSONArgument(value)
	if err != nil {
		return nil, err
	}
	var data interface{}
	if err := json.Unmarshal([]byte(resolved), &data); err != nil {
		return nil, fmt.Errorf("invalid --%s JSON: %w", flag, err)
	}
	return data, nil
}

func requireListArray(value, flag string) ([]interface{}, error) {
	data, err := parseListJSON(value, flag)
	if err != nil {
		return nil, err
	}
	array, ok := data.([]interface{})
	if !ok || len(array) == 0 {
		return nil, fmt.Errorf("--%s must be a non-empty JSON array", flag)
	}
	return array, nil
}

func runListsCreate(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	name := artifactFlagString(cmd, "name")
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	description, err := parseListJSON(artifactFlagString(cmd, "description-blocks"), "description-blocks")
	if err != nil {
		return err
	}
	if description != nil {
		if _, ok := description.([]interface{}); !ok {
			return fmt.Errorf("--description-blocks must be a JSON array")
		}
	}
	schema, err := parseListJSON(artifactFlagString(cmd, "schema"), "schema")
	if err != nil {
		return err
	}
	if schema != nil {
		if err := validateListSchema(schema); err != nil {
			return err
		}
	}
	include := cmd.Flags().Changed("include-copied-records")
	todo := cmd.Flags().Changed("todo-mode")
	includeValue, todoValue := cmdFlagBool(cmd, "include-copied-records"), cmdFlagBool(cmd, "todo-mode")
	result, err := ctx.Client.CreateSlackList(ctx.Ctx, appslack.ListCreateParams{Name: name, DescriptionBlocks: description, Schema: schema, CopyFromListID: artifactFlagString(cmd, "copy-from"), IncludeCopiedListRecords: optionalBool(include, includeValue), TodoMode: optionalBool(todo, todoValue)})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsUpdate(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	description, err := parseListJSON(artifactFlagString(cmd, "description-blocks"), "description-blocks")
	if err != nil {
		return err
	}
	if description != nil {
		if _, ok := description.([]interface{}); !ok {
			return fmt.Errorf("--description-blocks must be a JSON array")
		}
	}
	if artifactFlagString(cmd, "name") == "" && description == nil && !cmd.Flags().Changed("todo-mode") {
		return fmt.Errorf("at least one of --name, --description-blocks, or --todo-mode is required")
	}
	result, err := ctx.Client.UpdateSlackList(ctx.Ctx, appslack.ListUpdateParams{ListID: listID, Name: artifactFlagString(cmd, "name"), DescriptionBlocks: description, TodoMode: optionalBool(cmd.Flags().Changed("todo-mode"), cmdFlagBool(cmd, "todo-mode"))})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsItemAdd(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	fields, err := parseListJSON(artifactFlagString(cmd, "fields"), "fields")
	if err != nil {
		return err
	}
	if fields != nil {
		fields, err = normalizeListFields(ctx, listID, fields)
		if err != nil {
			return err
		}
	}
	result, err := ctx.Client.CreateSlackListItem(ctx.Ctx, appslack.ListItemCreateParams{ListID: listID, DuplicatedItemID: artifactFlagString(cmd, "duplicate"), ParentItemID: artifactFlagString(cmd, "parent"), InitialFields: fields})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsItemUpdate(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	itemID := artifactFlagString(cmd, "item")
	if itemID == "" {
		return fmt.Errorf("--item is required")
	}
	cells, err := requireListArray(artifactFlagString(cmd, "cells"), "cells")
	if err != nil {
		return err
	}
	cells, err = normalizeListFields(ctx, listID, cells)
	if err != nil {
		return err
	}
	for _, cell := range cells {
		object, ok := cell.(map[string]interface{})
		if !ok {
			return fmt.Errorf("--cells entries must be JSON objects")
		}
		object["row_id"] = itemID
	}
	result, err := ctx.Client.UpdateSlackListItem(ctx.Ctx, listID, cells)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsItemDelete(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	itemID := artifactFlagString(cmd, "item")
	if itemID == "" {
		return fmt.Errorf("--item is required")
	}
	result, err := ctx.Client.DeleteSlackListItem(ctx.Ctx, listID, itemID)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsItemsDelete(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	items, err := commaArtifactValues(artifactFlagString(cmd, "items"), "items")
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("--items is required")
	}
	result, err := ctx.Client.DeleteSlackListItems(ctx.Ctx, listID, items)
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsShare(cmd *cobra.Command, _ []string) error {
	return runListsAccess(cmd, false)
}

func runListsRevoke(cmd *cobra.Command, _ []string) error {
	return runListsAccess(cmd, true)
}

func runListsAccess(cmd *cobra.Command, revoke bool) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	channels, users, err := resolveArtifactTargets(cmd, ctx)
	if err != nil {
		return err
	}
	if len(channels) == 0 && len(users) == 0 {
		return fmt.Errorf("one of --channels or --users is required")
	}
	if len(channels) > 0 && len(users) > 0 {
		return fmt.Errorf("--channels and --users cannot be combined")
	}
	if revoke {
		result, err := ctx.Client.DeleteSlackListAccess(ctx.Ctx, listID, channels, users)
		if err != nil {
			return err
		}
		return printArtifactResponse(cmd, result)
	}
	level := artifactFlagString(cmd, "access-level")
	if level != "read" && level != "write" && level != "owner" {
		return fmt.Errorf("--access-level must be read, write, or owner")
	}
	if level == "owner" && len(channels) > 0 {
		return fmt.Errorf("owner access requires --users")
	}
	result, err := ctx.Client.SetSlackListAccess(ctx.Ctx, appslack.ListAccessParams{ListID: listID, AccessLevel: level, ChannelIDs: channels, UserIDs: users})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsExport(cmd *cobra.Command, _ []string) error {
	if artifactFlagString(cmd, "job") == "" {
		if artifactFlagString(cmd, "output") != "" {
			return fmt.Errorf("--output requires --job; start an export first, then retrieve it with --job")
		}
		return runListsExportStart(cmd, nil)
	}
	return runListsExportGet(cmd, nil)
}

func runListsExportStart(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	format := artifactFlagString(cmd, "format")
	if format != "csv" && format != "json" {
		return fmt.Errorf("--format must be csv or json")
	}
	if format == "csv" && (cmdFlagBool(cmd, "include-threads") || cmdFlagBool(cmd, "include-attachments")) {
		return fmt.Errorf("--include-threads and --include-attachments require --format json")
	}
	result, err := ctx.Client.StartSlackListDownload(ctx.Ctx, appslack.ListDownloadStartParams{ListID: listID, IncludeArchived: optionalBool(cmd.Flags().Changed("include-archived"), cmdFlagBool(cmd, "include-archived")), Format: format, IncludeThreads: optionalBool(cmd.Flags().Changed("include-threads"), cmdFlagBool(cmd, "include-threads")), IncludeAttachments: optionalBool(cmd.Flags().Changed("include-attachments"), cmdFlagBool(cmd, "include-attachments"))})
	if err != nil {
		return err
	}
	return printArtifactResponse(cmd, result)
}

func runListsExportGet(cmd *cobra.Command, _ []string) error {
	ctx, err := NewCommandContext(cmd, 0)
	if err != nil {
		return err
	}
	defer ctx.Close()
	listID, err := listIDFlag(cmd)
	if err != nil {
		return err
	}
	job := artifactFlagString(cmd, "job")
	if job == "" {
		return fmt.Errorf("--job is required")
	}
	format := artifactFlagString(cmd, "format")
	if format != "csv" && format != "json" {
		return fmt.Errorf("--format must be csv or json")
	}
	if format == "csv" && (cmdFlagBool(cmd, "include-threads") || cmdFlagBool(cmd, "include-attachments")) {
		return fmt.Errorf("--include-threads and --include-attachments require --format json")
	}
	result, err := ctx.Client.GetSlackListDownload(ctx.Ctx, appslack.ListDownloadGetParams{ListID: listID, JobID: job, Format: format, IncludeThreads: optionalBool(cmd.Flags().Changed("include-threads"), cmdFlagBool(cmd, "include-threads")), IncludeAttachments: optionalBool(cmd.Flags().Changed("include-attachments"), cmdFlagBool(cmd, "include-attachments"))})
	if err != nil {
		return err
	}
	outputPath := artifactFlagString(cmd, "output")
	if outputPath == "" {
		return printArtifactResponse(cmd, result)
	}
	var response struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(result, &response); err != nil || response.DownloadURL == "" {
		return fmt.Errorf("export response has no download_url; check job status")
	}
	if err := downloadArtifactURL(ctx.Ctx, ctx.Client, response.DownloadURL, outputPath, flagBool(cmd, "force")); err != nil {
		return err
	}
	return output.Print(cmd, map[string]interface{}{"ok": true, "list_id": listID, "job_id": job, "format": format, "output": filepath.Clean(outputPath)})
}

func optionalBool(set, value bool) *bool {
	if !set {
		return nil
	}
	return &value
}

func cmdFlagBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func validateListSchema(value interface{}) error {
	array, ok := value.([]interface{})
	if !ok || len(array) == 0 {
		return fmt.Errorf("--schema must be a non-empty JSON array")
	}
	allowed := map[string]bool{"text": true, "message": true, "number": true, "select": true, "date": true, "user": true, "attachment": true, "checkbox": true, "email": true, "phone": true, "channel": true, "rating": true, "created_by": true, "last_edited_by": true, "created_time": true, "last_edited_time": true, "vote": true, "canvas": true, "reference": true, "link": true, "rich_text": true}
	for i, item := range array {
		column, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("--schema entry %d must be an object", i)
		}
		for _, required := range []string{"key", "name", "type"} {
			value, exists := column[required].(string)
			if !exists || strings.TrimSpace(value) == "" {
				return fmt.Errorf("--schema entry %d requires %s", i, required)
			}
		}
		typeName := strings.ToLower(strings.TrimSpace(column["type"].(string)))
		if !allowed[typeName] {
			return fmt.Errorf("--schema entry %d has unsupported type %q", i, column["type"])
		}
	}
	return nil
}

// normalizeListFields accepts native Slack field/cell objects and the more
// agent-friendly {"column":"Name","value":...} form. Names and keys are
// resolved against the List schema; emitted requests always use column_id.
func normalizeListFields(ctx *CommandContext, listID string, data interface{}) ([]interface{}, error) {
	array, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("list fields must be a JSON array")
	}
	if len(array) == 0 {
		return array, nil
	}
	var schema map[string]listColumnDefinition
	loadSchema := func() error {
		if schema != nil {
			return nil
		}
		metadata, err := ctx.Client.GetSlackList(ctx.Ctx, listID)
		if err != nil {
			return err
		}
		schema, err = listSchema(metadata.File)
		return err
	}
	for i, item := range array {
		object, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("list field %d must be an object", i)
		}
		columnID, hasColumnID := object["column_id"].(string)
		if hasColumnID && strings.TrimSpace(columnID) != "" {
			continue
		}
		column := listStringField(object, "column")
		if column == "" {
			column = listStringField(object, "column_name")
		}
		if column == "" {
			column = listStringField(object, "column_key")
		}
		if column == "" {
			return nil, fmt.Errorf("list field %d requires column_id or column/column_name/column_key", i)
		}
		if err := loadSchema(); err != nil {
			return nil, err
		}
		definition, found := schema[strings.ToLower(column)]
		if !found || definition.ID == "" {
			if found && definition.ID == "" {
				return nil, fmt.Errorf("list column %q is ambiguous; use its column_id", column)
			}
			return nil, fmt.Errorf("list column %q was not found; use the column_id from lists items --list ...", column)
		}
		value, exists := object["value"]
		if !exists {
			return nil, fmt.Errorf("list field %d requires value when using column names", i)
		}
		var err error
		if definition.Type == "user" {
			value, err = resolveListUsers(ctx, value)
			if err != nil {
				return nil, err
			}
		}
		if definition.Type == "channel" {
			value, err = resolveListChannels(ctx, value)
			if err != nil {
				return nil, err
			}
		}
		delete(object, "column")
		delete(object, "column_name")
		delete(object, "column_key")
		delete(object, "value")
		object["column_id"] = definition.ID
		typed, err := typedListValue(definition.Type, value)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", column, err)
		}
		for key, typedValue := range typed {
			object[key] = typedValue
		}
	}
	return array, nil
}

type listColumnDefinition struct{ ID, Type string }

func listStringField(object map[string]interface{}, key string) string {
	value, ok := object[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func listSchema(file map[string]interface{}) (map[string]listColumnDefinition, error) {
	result := map[string]listColumnDefinition{}
	metadata, _ := file["list_metadata"].(map[string]interface{})
	columns, _ := metadata["schema"].([]interface{})
	for _, item := range columns {
		column, _ := item.(map[string]interface{})
		id, _ := column["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		definition := listColumnDefinition{ID: id, Type: strings.ToLower(fmt.Sprint(column["type"]))}
		for _, key := range []string{"name", "key", "id"} {
			value, _ := column[key].(string)
			if value = strings.TrimSpace(value); value != "" {
				mapKey := strings.ToLower(value)
				if previous, exists := result[mapKey]; exists && previous.ID != definition.ID {
					// Keep an explicit ambiguity marker. Native column_id requests can
					// bypass schema loading, while name-based requests fail clearly.
					result[mapKey] = listColumnDefinition{}
					continue
				}
				result[mapKey] = definition
			}
		}
	}
	return result, nil
}

func typedListValue(columnType string, value interface{}) (map[string]interface{}, error) {
	switch columnType {
	case "text", "rich_text":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("text value must be a string")
		}
		return map[string]interface{}{"rich_text": []interface{}{map[string]interface{}{"type": "rich_text", "elements": []interface{}{map[string]interface{}{"type": "rich_text_section", "elements": []interface{}{map[string]interface{}{"type": "text", "text": text}}}}}}}, nil
	case "select", "user", "channel", "date", "message", "email", "phone", "attachment":
		values, err := asStringArray(value)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{columnType: values}, nil
	case "link":
		if link, ok := value.(map[string]interface{}); ok {
			return map[string]interface{}{"link": []interface{}{link}}, nil
		}
		return map[string]interface{}{"link": []interface{}{map[string]interface{}{"original_url": fmt.Sprint(value)}}}, nil
	case "reference":
		return map[string]interface{}{"reference": value}, nil
	case "timestamp":
		if _, ok := value.(float64); !ok {
			return nil, fmt.Errorf("timestamp value must be a JSON number")
		}
		return map[string]interface{}{"timestamp": []interface{}{value}}, nil
	case "checkbox":
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("checkbox value must be boolean")
		}
		return map[string]interface{}{"checkbox": value}, nil
	case "number", "rating":
		if _, ok := value.(float64); !ok {
			return nil, fmt.Errorf("numeric value must be a JSON number")
		}
		key := columnType
		return map[string]interface{}{key: []interface{}{value}}, nil
	default:
		return nil, fmt.Errorf("column type %q does not have a supported shorthand; provide native typed field data with column_id", columnType)
	}
}

func resolveListUsers(ctx *CommandContext, value interface{}) (interface{}, error) {
	refs, err := asStringArray(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		id, err := ctx.ResolveUser(ref)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func resolveListChannels(ctx *CommandContext, value interface{}) (interface{}, error) {
	refs, err := asStringArray(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		id, err := ctx.ResolveChannel(ref)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func asStringArray(value interface{}) ([]string, error) {
	if values, ok := value.([]string); ok {
		result := make([]string, len(values))
		for i, item := range values {
			if strings.TrimSpace(item) == "" {
				return nil, fmt.Errorf("string array item %d is empty", i)
			}
			result[i] = item
		}
		return result, nil
	}
	if values, ok := value.([]interface{}); ok {
		result := make([]string, len(values))
		for i, item := range values {
			stringValue, ok := item.(string)
			if !ok || strings.TrimSpace(stringValue) == "" {
				return nil, fmt.Errorf("string array item %d must be a non-empty string", i)
			}
			result[i] = stringValue
		}
		return result, nil
	}
	stringValue, ok := value.(string)
	if !ok || strings.TrimSpace(stringValue) == "" {
		return nil, fmt.Errorf("value must be a non-empty string or string array")
	}
	return []string{stringValue}, nil
}

func downloadArtifactURL(ctx context.Context, client *appslack.APIClient, url, path string, force bool) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output destination already exists (use --force to replace it): %s", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect output destination: %w", err)
		}
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".slk-list-export-*")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = file.Close(); _ = os.Remove(temporary) }()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if err := client.DownloadFile(ctx, url, file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if force {
		return os.Rename(temporary, path)
	}
	if err := os.Link(temporary, path); err != nil {
		return err
	}
	return nil
}
