# slack-agent-cli

A CLI tool for AI coding agents to interact with Slack workspaces via command line.

## Project Overview

| Attribute | Value |
|-----------|-------|
| Language | Go |
| CLI Framework | Cobra + Viper |
| Slack SDK | slack-go/slack |
| Config Location | `~/.config/slack-agent-cli/config.json` |

## Quick Reference

```bash
# Development commands
go build ./...              # Build all packages
go test ./...               # Run all tests
go run main.go <command>    # Run CLI directly
go run main.go --help       # See available commands
```

## Documentation

- [docs/DESIGN.md](docs/DESIGN.md) - **READ THIS FIRST** for command specs, output formats, and API details

---

## Agent Architecture

You are the **master agent**. You orchestrate work by delegating to specialized subagents.

### Available Subagents

| Agent | Purpose | When to Use |
|-------|---------|-------------|
| `coder` | Implements features and fixes bugs | Delegate coding tasks |
| `explorer` | Investigates codebase (read-only) | Delegate research/debugging questions |
| `validator` | Validates implementations against design | After coder completes work |

### Orchestration Workflow

```
1. User requests a feature/fix
2. You (master) check `bd ready` for tasks or create a plan
3. You delegate to `@coder` with task ID and clear instructions
4. Coder implements and reports back (does NOT commit)
5. You delegate to `@validator` to verify against docs/DESIGN.md
6. If validation passes: you commit, push, and close the task
7. If validation fails: you delegate back to `@coder` with feedback
```

### Delegation Rules

**When delegating to subagents:**
1. Always provide the Beads task ID (if applicable)
2. Give clear, specific instructions about what to implement/investigate
3. Subagents will update `bd` with progress but will NOT:
   - Run `bd close` (you do this)
   - Run `git commit` or `git push` (you do this)

**After subagent completes:**
1. Review their output
2. If coder finished: delegate to validator
3. If validator passes: commit and push
4. If validator fails: delegate back to coder with specific fixes needed

### Your Responsibilities (Master Only)

- `bd close <task-id>` - Only you close tasks
- `git add`, `git commit`, `git push` - Only you handle git
- Final quality check before pushing
- Resolving conflicts between subagent outputs

---

## Issue Tracking with Beads (`bd`)

```bash
bd ready                              # Show tasks ready to work on
bd show <id>                          # View task details
bd update <id> --status in_progress   # Start a task
bd close <id>                         # Complete a task
```
If Beads is unavailable, track work via git commits and PR descriptions.

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
- **One source of truth.** The Beads task must reflect the current state of work (in progress, blocked, ready for review, etc.) and mention whether requirements are satisfied/tests are passing. Avoid silent progress—if it's done or tested, it must be recorded via `bd update` or `bd close`.

### Task Completion Criteria

**A task is ONLY complete when ALL quality gates pass:**

```bash
# Run this single command to verify all criteria
go fmt ./... && go vet ./... && go build ./... && go test ./...
```

| Criterion | Verification | Expected Output |
|-----------|-------------|-----------------|
| Compiles | `go build ./...` | No output (exit 0) |
| Tests pass | `go test ./...` | `ok` for each package |
| Formatted | `go fmt ./...` | No output (no changes) |
| No issues | `go vet ./...` | No output (exit 0) |

**Additional requirements:**
- New code has tests (aim for >80% coverage on new code)
- Changes are committed with a meaningful message
- If using Beads: `bd close <task-id>`

---

## Task-Specific Deliverables

**DISCOVERY FIRST**: Before writing any code, use tools to understand existing patterns. Do not rely on documentation alone—the codebase is the source of truth.

### For "Implement X command" tasks

**Step 1: Discover the pattern**
```bash
# Find the most similar existing command
go run main.go --help                    # What commands exist?
go run main.go <similar-command> --help  # How does it work?
```

Then READ the corresponding `cmd/<similar>.go` file to understand:
- How flags are defined
- How the `RunE` function is structured
- What internal packages it imports

**Step 2: Verify your implementation**
```bash
# MUST pass all checks
go run main.go --help | grep <your-command>     # Command registered
go run main.go <your-command> --help            # Flags documented
go run main.go <your-command> <valid-args>      # Runs without panic
go test ./cmd/... ./internal/<domain>/...       # Tests pass
```

### For "Implement X SDK integration" tasks

**Discovery**: Read `internal/slack/client.go` to understand the interface pattern, then read `internal/slack/mock_client.go` to see how mocking works.

**Verification**:
```bash
go test ./internal/slack/...   # All tests pass with mocks
```

### For "Implement configuration" tasks

**Discovery**: Read `internal/config/config.go` for the struct layout and `internal/config/config_test.go` for test patterns.

**Verification**:
```bash
go test ./internal/config/...  # All tests pass
```

---

## Project Structure

```
slack-agent-cli/
├── cmd/                    # Cobra commands
│   ├── root.go             # Root command, global flags
│   ├── auth.go             # auth subcommands
│   ├── cache.go            # cache subcommands
│   ├── messages.go         # messages subcommands
│   ├── channels.go         # channels subcommands
│   ├── reactions.go        # reactions subcommands
│   ├── pins.go             # pins subcommands
│   ├── users.go            # users subcommands
│   └── emoji.go            # emoji subcommands
├── internal/
│   ├── config/             # Configuration management
│   ├── slack/              # Slack API client wrapper
│   ├── output/             # Output formatting (JSON, human-readable)
│   ├── cache/              # Metadata caching
│   ├── channels/           # Channel resolution
│   ├── messages/           # Message operations
│   ├── users/              # User operations
│   └── usergroups/         # User group operations
├── docs/
│   └── DESIGN.md           # Design document
├── main.go                 # Entry point
├── go.mod
├── go.sum
└── AGENTS.md               # This file
```

---

## Code Conventions

**DISCOVER, DON'T MEMORIZE**: Patterns evolve. Always read the actual implementation files rather than relying on examples below.

### Command Pattern (Cobra)

**Discovery command:**
```bash
# Read the messages command as your template
cat cmd/messages.go
```

Key elements to replicate:
- `var <name>Cmd = &cobra.Command{...}` with Use, Short, Long, Example, RunE
- `func init()` that registers command and defines flags
- `func run<Name>(cmd *cobra.Command, args []string) error` as the handler

### Output Pattern

**Discovery command:**
```bash
cat internal/output/output.go
```

All commands must support both human-readable (default) and JSON (`--json`) output via the `output.Print()` function.

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

Every new function should have tests. **Discover the pattern by reading existing tests:**

```bash
# Find test files matching your domain
ls internal/*/*.go | grep _test.go

# Read a representative test file
cat internal/config/config_test.go
cat internal/channels/service_test.go
```

Key patterns to follow:
- Use table-driven tests (`tests := []struct{...}`)
- Use `t.Run(tt.name, func(t *testing.T) {...})` for subtests
- Use `t.TempDir()` for file system isolation
- Use `t.Setenv()` to override environment variables

### Integration Tests (when Slack credentials available)

```go
//go:build integration

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
go mod init github.com/user/slack-agent-cli
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
Config file missing or tokens invalid. Run `slack-agent-cli config init`.

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
