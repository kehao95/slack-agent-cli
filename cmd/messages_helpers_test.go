package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBlocksJSON_Empty(t *testing.T) {
	blocks, err := parseBlocksJSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocks != nil {
		t.Errorf("expected nil blocks, got %v", blocks)
	}
}

func TestParseBlocksJSON_ValidSection(t *testing.T) {
	input := `[{"type": "section", "text": {"type": "mrkdwn", "text": "Hello"}}]`
	blocks, err := parseBlocksJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}

func TestParseBlocksJSON_InvalidJSON(t *testing.T) {
	_, err := parseBlocksJSON("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseBlocksJSON_ForwardsUnknownType(t *testing.T) {
	input := `[{"type": "unknown_type"}]`
	blocks, err := parseBlocksJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	encoded, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	if string(encoded) != `[{"type":"unknown_type"}]` {
		t.Fatalf("unexpected forwarded JSON: %s", encoded)
	}
}

func TestParseBlocksJSON_MissingType(t *testing.T) {
	_, err := parseBlocksJSON(`[{"block_id":"missing"}]`)
	if err == nil {
		t.Fatal("expected missing type error")
	}
}

func TestParseBlocksJSON_MultipleBlocks(t *testing.T) {
	input := `[
        {"type": "header", "text": {"type": "plain_text", "text": "Title"}},
        {"type": "divider"},
        {"type": "section", "text": {"type": "mrkdwn", "text": "Body"}}
    ]`
	blocks, err := parseBlocksJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
}

func TestParseBlocksJSON_FromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocks.json")
	if err := os.WriteFile(path, []byte(`[{"type":"rich_text","elements":[]}]`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	blocks, err := parseBlocksJSON("@" + path)
	if err != nil {
		t.Fatalf("parseBlocksJSON: %v", err)
	}
	if len(blocks) != 1 || string(blocks[0].(rawBlock).raw) == "" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
}
