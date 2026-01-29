# slack-cli

A CLI tool for AI coding agents to interact with Slack workspaces via command line.

## Project Overview

| Attribute | Value |
|-----------|-------|
| Language | Go |
| CLI Framework | Cobra + Viper |
| Slack SDK | slack-go/slack |
| Issue Tracker | Beads (`bd`) |
| Config Location | `~/.config/slack-cli/config.json` |

## Quick Reference

```bash
# Issue tracking
bd ready                    # Show tasks ready to work on
bd show <id>                # View task details and acceptance criteria
bd update <id> --status in_progress   # Start a task
bd close <id>               # Complete a task

# Development
go build ./...              # Build
go test ./...               # Run tests
go run main.go <command>    # Run CLI
```

## Documentation

- [docs/DESIGN.md](docs/DESIGN.md) - Full design document with API specs

---

## Development Workflow

### Starting Work

1. **Check ready tasks**: `bd ready`
2. **Pick a task**: Choose highest priority unblocked task
3. **Read requirements**: `bd show <task-id>` for full details
4. **Mark in progress**: `bd update <task-id> --status in_progress`

### Task Tracking Discipline

- **Always keep Beads up to date.** Every time you begin coding, run `bd update <task-id> --status in_progress` (if not already) and add a short note on what you are tackling next.
- **Log progress after each meaningful change or test run.** Use `bd update <task-id> --notes "Progress: <summary>. Tests: go test ./..."` to capture what was implemented and which verification commands were executed.
- **One source of truth.** The Beads task must reflect the current state of work (in progress, blocked, ready for review, etc.) and mention whether requirements are satisfied/tests are passing. Avoid silent progress—if it’s done or tested, it must be recorded via `bd update` or `bd close`.

### Task Completion Criteria

**A task is ONLY complete when ALL of the following are true:**

1. ✅ Code compiles without errors: `go build ./...`
2. ✅ All tests pass: `go test ./...`
3. ✅ New code has tests (aim for >80% coverage on new code)
4. ✅ Code follows Go conventions: `go fmt ./...` and `go vet ./...`
5. ✅ The specific acceptance criteria in the task are met
6. ✅ Changes are committed with a meaningful message
7. ✅ Task is closed: `bd close <task-id>`

---

## Task-Specific Deliverables

Each task type has specific deliverables that MUST be completed:

### For "Implement X command" tasks

**Required deliverables:**
1. Command file: `cmd/<command>.go` or `cmd/<parent>/<subcommand>.go`
2. Business logic: `internal/<domain>/<feature>.go`
3. Tests: `internal/<domain>/<feature>_test.go`
4. Command registered in parent command's `init()`
5. Help text with examples: `Use`, `Short`, `Long`, `Example` fields

**Verification:**
```bash
# Command appears in help
go run main.go --help
go run main.go <parent> --help

# Command executes without panic (even if API fails without config)
go run main.go <command> --help
```

### For "Implement X SDK integration" tasks

**Required deliverables:**
1. Client wrapper: `internal/slack/client.go`
2. Interface definition for testability
3. Mock implementation: `internal/slack/mock_client.go`
4. Connection/auth validation function
5. Unit tests with mocks

**Verification:**
```bash
go test ./internal/slack/...
```

### For "Implement configuration" tasks

**Required deliverables:**
1. Config struct with JSON tags: `internal/config/config.go`
2. Load/Save functions with proper error handling
3. Validation function
4. Default values
5. Environment variable override support
6. Tests for load/save/validate

**Verification:**
```bash
go test ./internal/config/...
```

---

## Project Structure

```
slack-cli/
├── cmd/                    # Cobra commands
│   ├── root.go             # Root command, global flags
│   ├── config.go           # config subcommands
│   ├── auth.go             # auth subcommands
│   ├── watch.go            # watch command
│   ├── messages.go         # messages subcommands
│   ├── channels.go         # channels subcommands
│   ├── reactions.go        # reactions subcommands
│   ├── pins.go             # pins subcommands
│   ├── users.go            # users subcommands
│   ├── files.go            # files subcommands
│   └── emoji.go            # emoji subcommands
├── internal/
│   ├── config/             # Configuration management
│   ├── slack/              # Slack API client wrapper
│   ├── output/             # Output formatting (JSON, human-readable)
│   └── watch/              # Socket Mode event handling
├── pkg/                    # Public packages (if any)
├── docs/
│   └── DESIGN.md           # Design document
├── main.go                 # Entry point
├── go.mod
├── go.sum
└── AGENTS.md               # This file
```

---

## Code Conventions

### Command Pattern (Cobra)

```go
// cmd/messages.go
var messagesCmd = &cobra.Command{
    Use:   "messages",
    Short: "Message operations",
    Long:  `Send, list, edit, and delete Slack messages.`,
}

var messagesListCmd = &cobra.Command{
    Use:   "list",
    Short: "List messages from a channel",
    Long:  `Fetch message history from a Slack channel using conversations.history API.`,
    Example: `  # Get last 20 messages
  slack-cli messages list --channel "#general" --limit 20
  
  # Get messages as JSON
  slack-cli messages list --channel "#general" --json`,
    RunE: runMessagesList,
}

func init() {
    rootCmd.AddCommand(messagesCmd)
    messagesCmd.AddCommand(messagesListCmd)
    
    messagesListCmd.Flags().StringP("channel", "c", "", "Channel name or ID (required)")
    messagesListCmd.Flags().IntP("limit", "l", 50, "Maximum messages to return")
    messagesListCmd.Flags().Bool("json", false, "Output as JSON")
    messagesListCmd.MarkFlagRequired("channel")
}

func runMessagesList(cmd *cobra.Command, args []string) error {
    // Implementation
}
```

### Output Pattern

```go
// internal/output/output.go
type Formatter interface {
    Format(data interface{}) (string, error)
}

// Always support both formats
func PrintOutput(cmd *cobra.Command, data interface{}) error {
    jsonFlag, _ := cmd.Flags().GetBool("json")
    if jsonFlag {
        return printJSON(data)
    }
    return printHuman(data)
}
```

### Error Handling

```go
// Use wrapped errors with context
if err != nil {
    return fmt.Errorf("failed to fetch messages from %s: %w", channel, err)
}

// Map to exit codes (see docs/DESIGN.md section 10.1)
// 0=success, 1=general, 2=config, 3=auth, 4=rate-limit, 5=network, 6=permission, 7=not-found
```

---

## Testing Requirements

### Unit Tests

Every new function should have tests:

```go
// internal/config/config_test.go
func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name    string
        setup   func() string  // Returns temp config path
        want    *Config
        wantErr bool
    }{
        {
            name: "valid config",
            setup: func() string {
                // Create temp file with valid config
            },
            want: &Config{...},
        },
        {
            name: "missing file returns default",
            setup: func() string { return "/nonexistent" },
            want: DefaultConfig(),
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests (when Slack credentials available)

```go
// +build integration

func TestSlackClient_ListChannels(t *testing.T) {
    if os.Getenv("SLACK_BOT_TOKEN") == "" {
        t.Skip("SLACK_BOT_TOKEN not set")
    }
    // Test with real API
}
```

---

## Dependencies

### Required Go Modules

```go
require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    github.com/spf13/viper v1.18.0     // Configuration
    github.com/slack-go/slack v0.12.0  // Slack SDK
)
```

### Installing Dependencies

```bash
go mod init github.com/user/slack-cli
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/slack-go/slack@latest
```

---

## Quality Gates

Before marking any task complete, run:

```bash
# Format code
go fmt ./...

# Check for issues
go vet ./...

# Build
go build ./...

# Run tests
go test ./...

# (Optional) Check test coverage
go test -cover ./...
```

**All commands must pass with exit code 0.**

---

## Landing the Plane (Session Completion)

**When ending a work session**, complete ALL steps below. Work is NOT complete until `git push` succeeds.

### Mandatory Checklist

1. **Quality gates pass**
   ```bash
   go fmt ./... && go vet ./... && go build ./... && go test ./...
   ```

2. **Update task status**
   ```bash
   bd close <completed-task-id>
   # Or if not complete:
   bd update <task-id> --notes "Progress: <what was done>. Remaining: <what's left>"
   ```

3. **Commit and push**
   ```bash
   git add -A
   git commit -m "<type>: <description>"
   git pull --rebase
   bd sync
   git push
   ```

4. **Verify push succeeded**
   ```bash
   git status  # MUST show "Your branch is up to date with 'origin/main'"
   ```

### Commit Message Format

```
<type>: <short description>

<optional body explaining what and why>

Closes: SLACK-xxx
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

### Critical Rules

- ❌ NEVER stop before pushing - that leaves work stranded locally
- ❌ NEVER say "ready to push when you are" - YOU must push
- ✅ If push fails, resolve conflicts and retry until it succeeds
- ✅ Always run quality gates before committing

---

## Common Issues & Solutions

### "go: command not found"
Go is not installed or not in PATH. Install from https://go.dev/dl/

### "cannot find package"
Run `go mod tidy` to sync dependencies.

### "Slack API error: not_authed"
Config file missing or tokens invalid. Run `slack-cli config init`.

### Tests fail with "no test files"
Create `*_test.go` files in the package directory.

---

## Task Priority Guide

| Priority | Meaning | Action |
|----------|---------|--------|
| P1 | Critical path | Work on these first |
| P2 | Important | Work after P1 complete |
| P3 | Nice to have | Work after P2 complete |

Check blocked tasks: `bd blocked`
Check ready tasks: `bd ready`
