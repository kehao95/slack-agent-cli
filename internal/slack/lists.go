package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ListItemsParams wraps arguments for slackLists.items.list.
type ListItemsParams struct {
	ListID   string
	Limit    int
	Cursor   string
	Archived bool
}

// ListItemsResponse represents the subset of Slack Lists response fields used by the CLI.
type ListItemsResponse struct {
	OK               bool                     `json:"ok"`
	Items            []map[string]interface{} `json:"items"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// ListItemInfoParams wraps arguments for slackLists.items.info.
type ListItemInfoParams struct {
	ListID              string
	ID                  string
	IncludeIsSubscribed bool
}

// ListItemInfoResponse represents the subset of slackLists.items.info used by the CLI.
type ListItemInfoResponse struct {
	OK       bool                     `json:"ok"`
	List     map[string]interface{}   `json:"list"`
	Record   map[string]interface{}   `json:"record"`
	Subtasks []map[string]interface{} `json:"subtasks,omitempty"`
}

type slackListsErrorResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed,omitempty"`
	Provided string `json:"provided,omitempty"`
}

// ListSlackListItems fetches items from a Slack List via slackLists.items.list.
func (c *APIClient) ListSlackListItems(ctx context.Context, params ListItemsParams) (*ListItemsResponse, error) {
	if strings.TrimSpace(params.ListID) == "" {
		return nil, ErrListRequired
	}

	payload := map[string]interface{}{
		"list_id": params.ListID,
	}
	if params.Limit > 0 {
		payload["limit"] = params.Limit
	}
	if trimmed := strings.TrimSpace(params.Cursor); trimmed != "" {
		payload["cursor"] = trimmed
	}
	if params.Archived {
		payload["archived"] = true
	}

	respBody, err := c.postSlackListsMethod(ctx, "slackLists.items.list", payload)
	if err != nil {
		return nil, err
	}

	var result ListItemsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode slackLists.items.list response: %w", err)
	}
	if result.OK {
		return &result, nil
	}
	return nil, fmt.Errorf("slackLists.items.list failed")
}

// GetSlackListItem fetches a single record from a Slack List via slackLists.items.info.
func (c *APIClient) GetSlackListItem(ctx context.Context, params ListItemInfoParams) (*ListItemInfoResponse, error) {
	if strings.TrimSpace(params.ListID) == "" {
		return nil, ErrListRequired
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, ErrRecordRequired
	}

	payload := map[string]interface{}{
		"list_id": params.ListID,
		"id":      params.ID,
	}
	if params.IncludeIsSubscribed {
		payload["include_is_subscribed"] = true
	}

	respBody, err := c.postSlackListsMethod(ctx, "slackLists.items.info", payload)
	if err != nil {
		return nil, err
	}

	var result ListItemInfoResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode slackLists.items.info response: %w", err)
	}
	if result.OK {
		return &result, nil
	}
	return nil, fmt.Errorf("slackLists.items.info failed")
}

func (c *APIClient) postSlackListsMethod(ctx context.Context, method string, payload map[string]interface{}) ([]byte, error) {
	if c == nil || c.rawHTTPClient == nil {
		return nil, fmt.Errorf("lists client is not initialized")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}

	endpoint := strings.TrimRight(c.endpoint, "/") + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(c.cookie) != "" {
		req.Header.Set("Cookie", "d="+c.cookie)
	}

	resp, err := c.rawHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", method, err)
	}

	var apiErr slackListsErrorResponse
	if err := json.Unmarshal(respBody, &apiErr); err == nil && !apiErr.OK && apiErr.Error != "" {
		if apiErr.Needed != "" || apiErr.Provided != "" {
			return nil, fmt.Errorf("%s: %s (needed: %s provided: %s)", method, apiErr.Error, apiErr.Needed, apiErr.Provided)
		}
		return nil, fmt.Errorf("%s: %s", method, apiErr.Error)
	}

	return respBody, nil
}
