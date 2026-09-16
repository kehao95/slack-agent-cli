package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ArtifactResponse preserves the complete Slack response while allowing the
// command layer to render it as JSON or a readable raw response.
type ArtifactResponse json.RawMessage

func (r ArtifactResponse) MarshalJSON() ([]byte, error) { return []byte(r), nil }
func (r ArtifactResponse) Lines() []string              { return []string{string(r)} }

func artifactResponse(raw json.RawMessage, err error) (ArtifactResponse, error) {
	if err != nil {
		return nil, err
	}
	return ArtifactResponse(append(json.RawMessage(nil), raw...)), nil
}

func (c *APIClient) artifactCall(ctx context.Context, method string, payload map[string]interface{}) (ArtifactResponse, error) {
	resp, err := c.CallAPI(ctx, method, payload, CallAPIOptions{MaxRetries: 3})
	return artifactResponse(resp, err)
}

// CanvasCreateParams describes canvases.create and
// conversations.canvases.create. DocumentContent is the Slack
// document_content object, normally {"type":"markdown","markdown":"..."}.
type CanvasCreateParams struct {
	Title           string
	ChannelID       string
	DocumentContent map[string]interface{}
}

func (c *APIClient) CreateCanvas(ctx context.Context, params CanvasCreateParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{}
	if strings.TrimSpace(params.Title) != "" {
		payload["title"] = strings.TrimSpace(params.Title)
	}
	if params.DocumentContent != nil {
		payload["document_content"] = params.DocumentContent
	}
	if strings.TrimSpace(params.ChannelID) != "" {
		payload["channel_id"] = strings.TrimSpace(params.ChannelID)
	}
	return c.artifactCall(ctx, "canvases.create", payload)
}

// CanvasCreateChannelParams is explicit to avoid accidentally sending a
// standalone canvas request when creating a channel canvas.
type CanvasCreateChannelParams struct {
	ChannelID       string
	Title           string
	DocumentContent map[string]interface{}
}

func (c *APIClient) CreateChannelCanvas(ctx context.Context, params CanvasCreateChannelParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{"channel_id": strings.TrimSpace(params.ChannelID)}
	if strings.TrimSpace(params.Title) != "" {
		payload["title"] = strings.TrimSpace(params.Title)
	}
	if params.DocumentContent != nil {
		payload["document_content"] = params.DocumentContent
	}
	return c.artifactCall(ctx, "conversations.canvases.create", payload)
}

type CanvasEditParams struct {
	CanvasID string
	Changes  []interface{}
}

func (c *APIClient) EditCanvas(ctx context.Context, params CanvasEditParams) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "canvases.edit", map[string]interface{}{
		"canvas_id": strings.TrimSpace(params.CanvasID),
		"changes":   params.Changes,
	})
}

func (c *APIClient) DeleteCanvas(ctx context.Context, canvasID string) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "canvases.delete", map[string]interface{}{"canvas_id": strings.TrimSpace(canvasID)})
}

func (c *APIClient) LookupCanvasSections(ctx context.Context, canvasID string, criteria map[string]interface{}) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "canvases.sections.lookup", map[string]interface{}{
		"canvas_id": strings.TrimSpace(canvasID),
		"criteria":  criteria,
	})
}

type CanvasAccessParams struct {
	CanvasID    string
	AccessLevel string
	ChannelIDs  []string
	UserIDs     []string
}

func (c *APIClient) SetCanvasAccess(ctx context.Context, params CanvasAccessParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{
		"canvas_id":    strings.TrimSpace(params.CanvasID),
		"access_level": strings.TrimSpace(params.AccessLevel),
	}
	if len(params.ChannelIDs) > 0 {
		payload["channel_ids"] = params.ChannelIDs
	}
	if len(params.UserIDs) > 0 {
		payload["user_ids"] = params.UserIDs
	}
	return c.artifactCall(ctx, "canvases.access.set", payload)
}

func (c *APIClient) DeleteCanvasAccess(ctx context.Context, canvasID string, channelIDs, userIDs []string) (ArtifactResponse, error) {
	payload := map[string]interface{}{"canvas_id": strings.TrimSpace(canvasID)}
	if len(channelIDs) > 0 {
		payload["channel_ids"] = channelIDs
	}
	if len(userIDs) > 0 {
		payload["user_ids"] = userIDs
	}
	return c.artifactCall(ctx, "canvases.access.delete", payload)
}

// ListCanvases lists one page of canvas file metadata. Slack documents
// files.list(types=canvas) as the Canvas discovery API.
func (c *APIClient) ListCanvases(ctx context.Context, limit int, cursor string) (ArtifactResponse, error) {
	payload := map[string]interface{}{"types": "canvas"}
	if limit > 0 {
		payload["count"] = limit
	}
	if strings.TrimSpace(cursor) != "" {
		payload["page"] = strings.TrimSpace(cursor)
	}
	return c.artifactCall(ctx, "files.list", payload)
}

func (c *APIClient) GetCanvasInfo(ctx context.Context, canvasID string) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "files.info", map[string]interface{}{"file": strings.TrimSpace(canvasID)})
}

// DownloadCanvas downloads the backing Canvas file via the documented private
// file URL returned by files.info. It returns the URL used for diagnostics.
func (c *APIClient) DownloadCanvas(ctx context.Context, canvasID string, writer io.Writer) (string, error) {
	if strings.TrimSpace(canvasID) == "" {
		return "", fmt.Errorf("canvas ID is required")
	}
	if writer == nil {
		return "", fmt.Errorf("canvas download writer is required")
	}
	info, err := c.GetCanvasInfo(ctx, canvasID)
	if err != nil {
		return "", err
	}
	var envelope struct {
		File map[string]interface{} `json:"file"`
	}
	if err := json.Unmarshal(info, &envelope); err != nil {
		return "", fmt.Errorf("decode canvas file metadata: %w", err)
	}
	downloadURL := stringValue(envelope.File["url_private_download"])
	if downloadURL == "" {
		downloadURL = stringValue(envelope.File["url_private"])
	}
	if downloadURL == "" {
		return "", fmt.Errorf("canvas %s has no private download URL", canvasID)
	}
	if err := c.DownloadFile(ctx, downloadURL, writer); err != nil {
		return "", err
	}
	return downloadURL, nil
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// BookmarkParams are the common fields accepted by bookmarks.add/edit.
type BookmarkParams struct {
	ChannelID   string
	BookmarkID  string
	Title       string
	Type        string
	Link        string
	Emoji       string
	EntityID    string
	ParentID    string
	AccessLevel string
}

func (c *APIClient) ListBookmarks(ctx context.Context, channelID string) (ArtifactResponse, error) {
	payload := map[string]interface{}{"channel_id": strings.TrimSpace(channelID)}
	return c.artifactCall(ctx, "bookmarks.list", payload)
}

func (c *APIClient) AddBookmark(ctx context.Context, params BookmarkParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{
		"channel_id": strings.TrimSpace(params.ChannelID),
		"title":      strings.TrimSpace(params.Title),
		"type":       strings.TrimSpace(params.Type),
	}
	if strings.TrimSpace(params.Link) != "" {
		payload["link"] = strings.TrimSpace(params.Link)
	}
	if strings.TrimSpace(params.Emoji) != "" {
		payload["emoji"] = strings.TrimSpace(params.Emoji)
	}
	if strings.TrimSpace(params.EntityID) != "" {
		payload["entity_id"] = strings.TrimSpace(params.EntityID)
	}
	if strings.TrimSpace(params.ParentID) != "" {
		payload["parent_id"] = strings.TrimSpace(params.ParentID)
	}
	if strings.TrimSpace(params.AccessLevel) != "" {
		payload["access_level"] = strings.TrimSpace(params.AccessLevel)
	}
	return c.artifactCall(ctx, "bookmarks.add", payload)
}

func (c *APIClient) EditBookmark(ctx context.Context, params BookmarkParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{
		"channel_id":  strings.TrimSpace(params.ChannelID),
		"bookmark_id": strings.TrimSpace(params.BookmarkID),
	}
	if strings.TrimSpace(params.Title) != "" {
		payload["title"] = strings.TrimSpace(params.Title)
	}
	if strings.TrimSpace(params.Link) != "" {
		payload["link"] = strings.TrimSpace(params.Link)
	}
	if strings.TrimSpace(params.Emoji) != "" {
		payload["emoji"] = strings.TrimSpace(params.Emoji)
	}
	return c.artifactCall(ctx, "bookmarks.edit", payload)
}

func (c *APIClient) RemoveBookmark(ctx context.Context, channelID, bookmarkID, sectionID string) (ArtifactResponse, error) {
	payload := map[string]interface{}{}
	if strings.TrimSpace(channelID) != "" {
		payload["channel_id"] = strings.TrimSpace(channelID)
	}
	if strings.TrimSpace(bookmarkID) != "" {
		payload["bookmark_id"] = strings.TrimSpace(bookmarkID)
	}
	if strings.TrimSpace(sectionID) != "" {
		payload["quip_section_id"] = strings.TrimSpace(sectionID)
	}
	return c.artifactCall(ctx, "bookmarks.remove", payload)
}

// ListCreateParams captures the currently public slackLists.create fields.
type ListCreateParams struct {
	Name                     string
	DescriptionBlocks        interface{}
	Schema                   interface{}
	CopyFromListID           string
	IncludeCopiedListRecords *bool
	TodoMode                 *bool
}

func (c *APIClient) CreateSlackList(ctx context.Context, params ListCreateParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{"name": strings.TrimSpace(params.Name)}
	if params.DescriptionBlocks != nil {
		payload["description_blocks"] = params.DescriptionBlocks
	}
	if params.Schema != nil {
		payload["schema"] = params.Schema
	}
	if strings.TrimSpace(params.CopyFromListID) != "" {
		payload["copy_from_list_id"] = strings.TrimSpace(params.CopyFromListID)
	}
	if params.IncludeCopiedListRecords != nil {
		payload["include_copied_list_records"] = *params.IncludeCopiedListRecords
	}
	if params.TodoMode != nil {
		payload["todo_mode"] = *params.TodoMode
	}
	return c.artifactCall(ctx, "slackLists.create", payload)
}

type ListUpdateParams struct {
	ListID            string
	Name              string
	DescriptionBlocks interface{}
	TodoMode          *bool
}

func (c *APIClient) UpdateSlackList(ctx context.Context, params ListUpdateParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{"id": strings.TrimSpace(params.ListID)}
	if strings.TrimSpace(params.Name) != "" {
		payload["name"] = strings.TrimSpace(params.Name)
	}
	if params.DescriptionBlocks != nil {
		payload["description_blocks"] = params.DescriptionBlocks
	}
	if params.TodoMode != nil {
		payload["todo_mode"] = *params.TodoMode
	}
	return c.artifactCall(ctx, "slackLists.update", payload)
}

type ListItemCreateParams struct {
	ListID           string
	DuplicatedItemID string
	ParentItemID     string
	InitialFields    interface{}
}

func (c *APIClient) CreateSlackListItem(ctx context.Context, params ListItemCreateParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{"list_id": strings.TrimSpace(params.ListID)}
	if strings.TrimSpace(params.DuplicatedItemID) != "" {
		payload["duplicated_item_id"] = strings.TrimSpace(params.DuplicatedItemID)
	}
	if strings.TrimSpace(params.ParentItemID) != "" {
		payload["parent_item_id"] = strings.TrimSpace(params.ParentItemID)
	}
	if params.InitialFields != nil {
		payload["initial_fields"] = params.InitialFields
	}
	return c.artifactCall(ctx, "slackLists.items.create", payload)
}

func (c *APIClient) UpdateSlackListItem(ctx context.Context, listID string, cells interface{}) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "slackLists.items.update", map[string]interface{}{
		"list_id": strings.TrimSpace(listID),
		"cells":   cells,
	})
}

func (c *APIClient) DeleteSlackListItem(ctx context.Context, listID, itemID string) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "slackLists.items.delete", map[string]interface{}{
		"list_id": strings.TrimSpace(listID),
		"id":      strings.TrimSpace(itemID),
	})
}

func (c *APIClient) DeleteSlackListItems(ctx context.Context, listID string, itemIDs []string) (ArtifactResponse, error) {
	return c.artifactCall(ctx, "slackLists.items.deleteMultiple", map[string]interface{}{
		"list_id": strings.TrimSpace(listID),
		"ids":     itemIDs,
	})
}

type ListAccessParams struct {
	ListID      string
	AccessLevel string
	ChannelIDs  []string
	UserIDs     []string
}

func (c *APIClient) SetSlackListAccess(ctx context.Context, params ListAccessParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{
		"list_id":      strings.TrimSpace(params.ListID),
		"access_level": strings.TrimSpace(params.AccessLevel),
	}
	if len(params.ChannelIDs) > 0 {
		payload["channel_ids"] = params.ChannelIDs
	}
	if len(params.UserIDs) > 0 {
		payload["user_ids"] = params.UserIDs
	}
	return c.artifactCall(ctx, "slackLists.access.set", payload)
}

func (c *APIClient) DeleteSlackListAccess(ctx context.Context, listID string, channelIDs, userIDs []string) (ArtifactResponse, error) {
	payload := map[string]interface{}{"list_id": strings.TrimSpace(listID)}
	if len(channelIDs) > 0 {
		payload["channel_ids"] = channelIDs
	}
	if len(userIDs) > 0 {
		payload["user_ids"] = userIDs
	}
	return c.artifactCall(ctx, "slackLists.access.delete", payload)
}

type ListDownloadStartParams struct {
	ListID             string
	IncludeArchived    *bool
	Format             string
	IncludeThreads     *bool
	IncludeAttachments *bool
}

func (c *APIClient) StartSlackListDownload(ctx context.Context, params ListDownloadStartParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{"list_id": strings.TrimSpace(params.ListID)}
	if params.IncludeArchived != nil {
		payload["include_archived"] = *params.IncludeArchived
	}
	if strings.TrimSpace(params.Format) != "" {
		payload["format"] = strings.TrimSpace(params.Format)
	}
	if params.IncludeThreads != nil {
		payload["include_threads"] = *params.IncludeThreads
	}
	if params.IncludeAttachments != nil {
		payload["include_attachments"] = *params.IncludeAttachments
	}
	return c.artifactCall(ctx, "slackLists.download.start", payload)
}

type ListDownloadGetParams struct {
	ListID             string
	JobID              string
	Format             string
	IncludeThreads     *bool
	IncludeAttachments *bool
}

func (c *APIClient) GetSlackListDownload(ctx context.Context, params ListDownloadGetParams) (ArtifactResponse, error) {
	payload := map[string]interface{}{
		"list_id": strings.TrimSpace(params.ListID),
		"job_id":  strings.TrimSpace(params.JobID),
	}
	if strings.TrimSpace(params.Format) != "" {
		payload["format"] = strings.TrimSpace(params.Format)
	}
	if params.IncludeThreads != nil {
		payload["include_threads"] = *params.IncludeThreads
	}
	if params.IncludeAttachments != nil {
		payload["include_attachments"] = *params.IncludeAttachments
	}
	return c.artifactCall(ctx, "slackLists.download.get", payload)
}
