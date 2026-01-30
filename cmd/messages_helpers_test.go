package cmd

import "testing"

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

func TestParseBlocksJSON_UnsupportedType(t *testing.T) {
	input := `[{"type": "unknown_type"}]`
	_, err := parseBlocksJSON(input)
	if err == nil {
		t.Error("expected error for unsupported block type")
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
