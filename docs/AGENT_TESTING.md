# Agent Testing Standards for slack-agent-cli

## Philosophy

This CLI is designed for **AI agents** to interact with Slack. All output must be machine-parseable, predictable, and scriptable. The testing strategy reflects this machine-first design.

**Core Testing Principles:**
1. **JSON-First Validation**: Default output is JSON. Test both JSON structure and human-readable fallback.
2. **Exit Code Precision**: Every error maps to a specific exit code (see DESIGN.md section 10.1).
3. **Stdout/Stderr Separation**: Data goes to stdout, status messages to stderr.
4. **No Surprises**: Commands never prompt, never block, never require interaction.

---

## Testing Checklist

### 1. JSON Output Validation

**Every command that produces data MUST:**
- [ ] Output valid JSON by default (parseable by `json.Unmarshal`)
- [ ] Have a consistent JSON structure documented in tests
- [ ] Include `ok: true` on success, `ok: false` on error
- [ ] Errors include `error.code` and `error.message` fields

**Test pattern:**
```go
func TestCommandJSONOutput(t *testing.T) {
    // Run command
    output := runCommand(t, "messages", "list", "--channel", "C123", "--limit", "5")
    
    // Validate JSON structure
    var result messages.ListResult
    if err := json.Unmarshal([]byte(output), &result); err != nil {
        t.Fatalf("invalid JSON: %v", err)
    }
    
    // Validate required fields
    if result.Channel == "" {
        t.Error("missing channel field")
    }
    if result.Messages == nil {
        t.Error("messages field is nil")
    }
}
```

### 2. Exit Codes (per DESIGN.md section 10.1)

| Code | Meaning | Test Coverage Required |
|------|---------|----------------------|
| 0 | Success | All happy path tests |
| 1 | General error | Unexpected failures |
| 2 | Config error | Missing/invalid config file |
| 3 | Auth error | Invalid/expired tokens |
| 4 | Rate limit | API rate limit exceeded |
| 5 | Network error | Connection failures |
| 6 | Permission denied | Missing OAuth scopes |
| 7 | Not found | Channel/user/message not found |

**Test pattern:**
```go
func TestExitCode_ConfigError(t *testing.T) {
    cmd := exec.Command("slack-agent-cli", "messages", "list", "--channel", "C123")
    cmd.Env = append(os.Environ(), "SLACK_CLI_CONFIG=/nonexistent/config.json")
    
    err := cmd.Run()
    if exitErr, ok := err.(*exec.ExitError); ok {
        if exitErr.ExitCode() != 2 {
            t.Errorf("expected exit code 2 (config error), got %d", exitErr.ExitCode())
        }
    } else {
        t.Error("expected command to fail with exit code 2")
    }
}
```

### 3. Stdout/Stderr Separation

**Rules:**
- `stdout`: ONLY data (JSON by default, tables with `--human`)
- `stderr`: Status messages, warnings, progress indicators
- `stderr`: Error messages (but also return as JSON in stdout)

**Test pattern:**
```go
func TestStdoutStderrSeparation(t *testing.T) {
    cmd := exec.Command("slack-agent-cli", "messages", "list", "--channel", "C123")
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    cmd.Run()
    
    // Stdout should be pure JSON
    var result map[string]interface{}
    if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
        t.Errorf("stdout is not valid JSON: %v", err)
    }
    
    // Stderr may contain status/debug messages (or be empty)
    // Should NOT contain data
    if strings.Contains(stderr.String(), `"channel"`) {
        t.Error("stderr contains data that should be in stdout")
    }
}
```

### 4. No Interactive Elements

**Prohibited patterns:**
- ❌ Reading from stdin without pipe detection
- ❌ Prompting for confirmation (use `--force` flag instead)
- ❌ Spinners or progress bars (use JSON progress events)
- ❌ Color codes in default output (only with `--human` flag)

**Test pattern:**
```go
func TestNoInteractivePrompts(t *testing.T) {
    // Commands should never block waiting for input
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, "slack-agent-cli", "messages", "delete", "--channel", "C123", "--ts", "123.456")
    
    // Close stdin immediately
    cmd.Stdin = nil
    
    err := cmd.Run()
    
    // Should not timeout waiting for input
    if ctx.Err() == context.DeadlineExceeded {
        t.Fatal("command blocked waiting for input - this is prohibited for agent CLIs")
    }
}
```

---

## Agent Workflow Test Examples

### Example 1: Read-Process-Reply Workflow

An agent monitors a channel and replies to messages.

```go
func TestAgentWorkflow_MonitorAndReply(t *testing.T) {
    // 1. List recent messages
    listOutput := runCommand(t, "messages", "list", "--channel", "#support", "--limit", "10")
    var listResult messages.ListResult
    json.Unmarshal([]byte(listOutput), &listResult)
    
    // 2. Agent processes messages
    var targetTS string
    for _, msg := range listResult.Messages {
        if strings.Contains(msg.Text, "help") {
            targetTS = msg.TS
            break
        }
    }
    
    // 3. Reply in thread
    if targetTS != "" {
        replyOutput := runCommand(t, "messages", "send",
            "--channel", "#support",
            "--thread", targetTS,
            "--text", "I can help with that!")
        
        var replyResult slack.PostMessageResult
        json.Unmarshal([]byte(replyOutput), &replyResult)
        
        if !replyResult.OK {
            t.Error("reply failed")
        }
    }
}
```

### Example 2: Search-React Workflow

An agent searches for errors and adds reactions.

```go
func TestAgentWorkflow_SearchAndReact(t *testing.T) {
    // 1. Search for error messages
    searchOutput := runCommand(t, "messages", "search",
        "--query", "error in:#engineering",
        "--limit", "5")
    
    var searchResult slack.SearchResult
    json.Unmarshal([]byte(searchOutput), &searchResult)
    
    // 2. Add reactions to each error message
    for _, match := range searchResult.Matches {
        reactOutput := runCommand(t, "reactions", "add",
            "--channel", match.Channel.ID,
            "--ts", match.TS,
            "--emoji", "eyes")
        
        var reactResult slack.ReactionResult
        json.Unmarshal([]byte(reactOutput), &reactResult)
        
        if !reactResult.OK {
            t.Errorf("failed to react to message %s", match.TS)
        }
    }
}
```

### Example 3: Batch Operations with Cache

An agent lists multiple channels efficiently using cache.

```go
func TestAgentWorkflow_BatchWithCache(t *testing.T) {
    // 1. Pre-warm cache (optional but recommended)
    runCommand(t, "cache", "populate", "channels", "--all")
    
    // 2. Check cache status
    statusOutput := runCommand(t, "cache", "status")
    var status cache.StatusResult
    json.Unmarshal([]byte(statusOutput), &status)
    
    if !status.Channels.Complete {
        t.Error("cache population incomplete")
    }
    
    // 3. Now channel name resolution is instant
    channels := []string{"#general", "#engineering", "#support"}
    for _, ch := range channels {
        // This uses cached data - no API calls
        listOutput := runCommand(t, "messages", "list", "--channel", ch, "--limit", "1")
        var result messages.ListResult
        json.Unmarshal([]byte(listOutput), &result)
        
        if !result.OK {
            t.Errorf("failed to list %s", ch)
        }
    }
}
```

---

## Testing Infrastructure

### Required Test Utilities

**Location:** `internal/testutil/testutil.go`

**Utilities needed:**
1. `CaptureOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int)`
2. `ValidateJSON(t *testing.T, data string) map[string]interface{}`
3. `MockConfig(t *testing.T, token string) string` - Creates temp config file
4. `MockSlackAPI(t *testing.T) *httptest.Server` - Mock Slack API server

### Smoke Test Coverage

**Location:** `cmd/smoke_test.go`

**Test every command for:**
1. Help text exists (`--help` doesn't panic)
2. Required flags enforced (missing flags = error)
3. Invalid flags rejected (unknown flags = error)
4. JSON output is parseable (when data is produced)

---

## Coverage Targets

| Package | Current | Target | Priority |
|---------|---------|--------|----------|
| `cmd/*` | 0% | 60% | HIGH - smoke tests needed |
| `internal/cache` | 37% | 80% | MEDIUM - error cases |
| `internal/channels` | 87% | 90% | LOW - already good |
| `internal/config` | 63% | 80% | MEDIUM - edge cases |
| `internal/messages` | 55% | 80% | HIGH - core functionality |
| `internal/output` | 0% | 90% | HIGH - JSON/human validation |
| `internal/slack` | 25% | 70% | HIGH - API interaction |
| `internal/users` | 80% | 85% | LOW - already good |

**Note:** 100% coverage is NOT the goal. Focus on:
- All error paths tested
- All public APIs tested
- Edge cases documented

---

## Running Tests

### Full Test Suite
```bash
go test ./... -v
```

### With Coverage
```bash
go test ./... -cover
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Smoke Tests Only
```bash
go test ./cmd/... -v -run TestSmoke
```

### Agent Workflow Tests
```bash
go test ./cmd/... -v -run TestAgentWorkflow
```

### Integration Tests (requires real Slack token)
```bash
export SLACK_USER_TOKEN=xoxp-...
go test ./... -v -tags=integration
```

---

## Pre-Refactoring Checklist

Before ANY refactoring work:

- [ ] All existing tests pass (`go test ./...`)
- [ ] Coverage baseline captured (`go test ./... -cover > baseline.txt`)
- [ ] Smoke tests exist for all commands
- [ ] At least one agent workflow test exists
- [ ] Exit codes are tested
- [ ] JSON output validation exists

After refactoring:

- [ ] All tests still pass
- [ ] Coverage didn't decrease
- [ ] New code has tests
- [ ] Agent workflow tests still work

---

## Anti-Patterns to Avoid

### ❌ Testing Implementation Details
```go
// BAD: Testing internal function names
func TestInternalParseTimestamp(t *testing.T) {
    result := parseTimestamp("1h") // internal function
    // ...
}
```

### ✅ Testing Public API Behavior
```go
// GOOD: Testing public command behavior
func TestMessagesListSinceFlag(t *testing.T) {
    result := runMessagesList(t, "--channel", "C123", "--since", "1h")
    // Verify behavior, not implementation
}
```

### ❌ Mocking Everything
```go
// BAD: Overly complex mocks
type MockSlackClientWithHistory struct {
    GetConversationHistoryFunc func(...) (...)
    GetConversationRepliesFunc func(...) (...)
    GetUserInfoFunc func(...) (...)
    // 20 more methods...
}
```

### ✅ Using Simple Test Doubles
```go
// GOOD: Simple interface-based mocks
type MockSlackClient struct {
    slack.Client
    HistoryResponse *slack.History
    HistoryError    error
}

func (m *MockSlackClient) GetConversationHistory(...) (*slack.History, error) {
    return m.HistoryResponse, m.HistoryError
}
```

---

## Continuous Validation

### Pre-Commit Hook (Recommended)
```bash
#!/bin/bash
# .git/hooks/pre-commit

go fmt ./...
go vet ./...
go test ./... -short

if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi
```

### CI Pipeline Requirements
```yaml
# .github/workflows/test.yml
- name: Run tests
  run: |
    go test ./... -v -race -coverprofile=coverage.out
    go tool cover -func=coverage.out
    
- name: Check coverage
  run: |
    coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    if (( $(echo "$coverage < 60" | bc -l) )); then
      echo "Coverage $coverage% is below 60%"
      exit 1
    fi
```

---

## Questions?

For questions about testing strategy, ask in the task's Beads comments or reference:
- [DESIGN.md](./DESIGN.md) - Command specifications and output formats
- [AGENTS.md](../AGENTS.md) - Development workflow and quality gates
