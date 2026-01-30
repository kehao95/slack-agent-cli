# Slack Agent CLI Design Document

## Slack for Non-Humans™

## 1. Executive Summary

A **machine-first** command-line interface for Slack. Designed for scripts, cron jobs, and AI agents. Humans are supported as second-class citizens.

**Core Philosophy:**
- **JSON First**: Default output is JSON. Human-readable output requires `--human` flag.
- **Pipe Friendly**: stdout contains only data. No progress messages, no "Success!", no spinners.
- **Stderr for Status**: All warnings, progress, and status messages go to stderr.
- **Non-Interactive**: No prompts, no confirmations. Fail fast with clear error codes.

**Key Design Decisions:**
- **User Token authentication** for acting as yourself in Slack
- **Batch operations** via `messages list` for history and `messages search` for queries
- **Single workspace** per configuration
- **Config file** for auth storage (`~/.config/slack-agent-cli/config.json`)
- **JSON default output**, `--human/-H` flag for human-readable tables

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User / Agent                            │
│               (Human / Claude Code / OpenCode)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Subprocess / Shell exec
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         slack-agent-cli                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ messages │  │ channels │  │reactions │  │    pins      │   │
│  │  (list)  │  │  (list)  │  │          │  │              │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘   │
│        │              │            │              │             │
│        └──────────────┴────────────┴──────────────┘             │
│                              │                                  │
│                    ┌─────────▼─────────┐                       │
│                    │   Slack SDK       │                       │
│                    │  (slack-go)       │                       │
│                    └─────────┬─────────┘                       │
└──────────────────────────────│──────────────────────────────────┘
                               │
                               │ Web API (User Token)
                               ▼
                    ┌─────────────────────┐
                    │    Slack Platform   │
                    └─────────────────────┘
```

---

## 3. Command Structure

### 3.1 Top-Level Commands

```
slack-agent-cli
├── config          # Configuration management
│   ├── init        # Interactive setup wizard
│   ├── show        # Display current config
│   └── set         # Set config values
│
├── auth            # Authentication
│   ├── test        # Verify credentials work
│   └── whoami      # Show current user info
│
├── cache           # Cache management (for name resolution)
│   ├── populate    # Fetch and cache channels/users
│   ├── status      # Show cache state
│   └── clear       # Clear cached data
│
├── channels        # Channel operations
│   ├── list        # List accessible channels
│   ├── info        # Get channel details
│   ├── join        # Join a channel
│   └── leave       # Leave a channel
│
├── messages        # Message operations
│   ├── list        # Fetch message history (batch)
│   ├── send        # Send a message
│   ├── reply       # Reply in thread
│   ├── edit        # Edit a message
│   ├── delete      # Delete a message
│   └── search      # Search messages
│
├── reactions       # Reaction operations
│   ├── add         # Add reaction to message
│   ├── remove      # Remove reaction
│   └── list        # List reactions on message
│
├── pins            # Pin operations
│   ├── add         # Pin a message
│   ├── remove      # Unpin a message
│   └── list        # List pinned messages
│
├── users           # User operations
│   ├── list        # List workspace members
│   ├── info        # Get user details
│   └── presence    # Check user presence
│
├── files           # File operations
│   ├── upload      # Upload a file
│   ├── download    # Download a file
│   └── list        # List files
│
└── emoji           # Emoji operations
    └── list        # List custom emoji
```

---

### 3.2 Command Details

#### `slack-agent-cli config init`

Interactive setup wizard to configure the CLI.

```bash
$ slack-agent-cli config init

Slack CLI Configuration
=======================

1. Create a Slack app at https://api.slack.com/apps
2. Add User Token Scopes and install to workspace
3. Copy the User Token (xoxp-...)

? User Token (xoxp-...): xoxp-123-456-abc...

Testing connection... ✓
User: alice
Workspace: My Workspace

Configuration saved to ~/.config/slack-agent-cli/config.json
```

---

#### `slack-agent-cli messages list`

Fetch message history using Slack's conversations.history API.

```bash
slack-agent-cli messages list [options]

Options:
  --channel <name|id>    Channel to fetch from (required)
  --limit <n>            Max messages to return (default: 50, max: 1000)
  --since <time>         Messages after this time (ISO 8601 or relative: "1h", "2d")
  --until <time>         Messages before this time
  --thread <ts>          Fetch replies in a specific thread
  --include-bots         Include bot messages (default: true)
  --refresh-cache        Force refresh of cached channel/user metadata before running
  --json                 Output as JSON
```

**Example:**
```bash
# Get last 20 messages from #general
slack-agent-cli messages list --channel "#general" --limit 20

# Get messages from the last hour
slack-agent-cli messages list --channel "#general" --since 1h --json

# Get thread replies
slack-agent-cli messages list --channel "#general" --thread "1705312365.000100"
```

After the first invocation warms the cache, subsequent `messages list` commands reuse the stored channel and user maps so resolution becomes effectively instantaneous unless `--refresh-cache` is specified.

---

#### `slack-agent-cli messages send`

Send a message to a channel or user.

```bash
slack-agent-cli messages send [options]

Options:
  --channel <name|id>    Target channel (use @user for DM)
  --text <message>       Message text (can also be piped via stdin)
  --thread <ts>          Reply in thread
  --blocks <json>        Block Kit JSON (for rich formatting)
  --unfurl-links         Unfurl URLs (default: true)
  --unfurl-media         Unfurl media (default: true)
  --json                 Output sent message details as JSON
```

**Examples:**
```bash
# Simple message
slack-agent-cli messages send --channel "#general" --text "Hello from CLI!"

# Reply in thread
slack-agent-cli messages send --channel "#general" --thread "1705312365.000100" --text "Thread reply"

# Pipe message content
echo "Multi-line\nmessage" | slack-agent-cli messages send --channel "#general"

# Send to user DM
slack-agent-cli messages send --channel "@alice" --text "Private message"
```

---

#### `slack-agent-cli messages search`

Search messages across the workspace.

```bash
slack-agent-cli messages search [options]

Options:
  --query <text>         Search query (required)
  --limit <n>            Max results to return (default: 20)
  --sort <field>         Sort by 'score' or 'timestamp' (default: timestamp)
  --sort-dir <dir>       Sort direction 'asc' or 'desc' (default: desc)
  --json                 Output as JSON
```

**Examples:**
```bash
# Basic search
slack-agent-cli messages search --query "deployment failed"

# Search with advanced syntax
slack-agent-cli messages search --query "from:@alice in:#general"

# Search and sort by relevance
slack-agent-cli messages search --query "error" --sort score --limit 20
```

---

#### `slack-agent-cli reactions add/remove/list`

Manage emoji reactions.

```bash
# Add reaction
slack-agent-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# Remove reaction
slack-agent-cli reactions remove --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# List reactions on a message
slack-agent-cli reactions list --channel "#general" --ts "1705312365.000100" --json
```

---

## 4. Configuration

### 4.1 Config File Location

```
~/.config/slack-agent-cli/config.json
```

Or via `SLACK_CLI_CONFIG` environment variable.

### 4.2 Persistent Cache Location

```
~/.config/slack-agent-cli/cache/
```

- Separate JSON files per domain (e.g., `channels.json`, `users.json`).
- Stored alongside the config directory with directories created at `0700` and files at `0600` permissions.
- Each file contains a payload of Slack metadata plus a `fetched_at` ISO 8601 timestamp used for TTL checks.
- Cache entries default to a 7-day TTL and are refreshed automatically when stale or when commands are invoked with `--refresh-cache`.
- Any command that mutates Slack state (e.g., channel creation) must invalidate affected cache files to prevent stale reads.

### 4.3 Cache Population Commands

The `cache` command group provides explicit control over metadata caching with incremental pagination support. This design allows AI agents to:
1. Control when API calls are made (no surprise rate limits)
2. Resume interrupted fetches without losing progress
3. Monitor cache state before running other commands

#### `slack-agent-cli cache populate`

Fetch and cache channels or users from Slack with incremental pagination.

```bash
slack-agent-cli cache populate <channels|users> [options]

Options:
  --all                  Fetch all pages (default: fetch one page)
  --page-delay <dur>     Delay between pages to avoid rate limits (default: 1s)
  --page-size <n>        Items per page (default: 200, max: 1000)
  --json                 Output progress as JSON
```

**Incremental Behavior:**
- Without `--all`: Fetches one page and saves progress (cursor) to partial cache
- With `--all`: Continues fetching until complete, saving after each page
- If interrupted, the next run resumes from the last saved cursor
- Once complete, partial cache is promoted to main cache (7-day TTL)

**Examples:**
```bash
# Fetch channels incrementally (one page at a time)
slack-agent-cli cache populate channels
slack-agent-cli cache populate channels  # Continues from cursor
slack-agent-cli cache populate channels  # Continues until done

# Or fetch all at once with rate limiting
slack-agent-cli cache populate channels --all --page-delay 2s

# Populate users cache
slack-agent-cli cache populate users --all
```

**Output (Human-readable):**
```
Fetching channels... page 1 (200 items, cursor: dXNlcl9...)
Fetching channels... page 2 (150 items, complete)
Cache populated: 350 channels
```

**Output (JSON):**
```json
{"status":"fetching","page":1,"count":200,"cursor":"dXNlcl9..."}
{"status":"fetching","page":2,"count":150,"cursor":""}
{"status":"complete","total":350}
```

#### `slack-agent-cli cache status`

Show current cache state.

```bash
slack-agent-cli cache status [options]

Options:
  --json                 Output as JSON
```

**Output (Human-readable):**
```
Cache Status
────────────────────────────────────────────
channels:  350 items, fetched 2024-01-15 10:00:00 (complete)
users:     125 items, fetched 2024-01-15 09:30:00 (partial, cursor: dXNlcl9...)
```

**Output (JSON):**
```json
{
  "channels": {
    "count": 350,
    "fetched_at": "2024-01-15T10:00:00Z",
    "complete": true,
    "expires_at": "2024-01-22T10:00:00Z"
  },
  "users": {
    "count": 125,
    "fetched_at": "2024-01-15T09:30:00Z",
    "complete": false,
    "next_cursor": "dXNlcl9..."
  }
}
```

#### `slack-agent-cli cache clear`

Clear cached data.

```bash
slack-agent-cli cache clear [channels|users]

# Clear all caches
slack-agent-cli cache clear

# Clear specific cache
slack-agent-cli cache clear channels
slack-agent-cli cache clear users
```

### 4.4 Cache and Channel Resolution

Commands that accept `--channel` support two formats:
1. **Direct ID** (`C074S0L3MCG`): Works immediately, no cache needed
2. **Channel name** (`#general`): Uses lazy-fetch strategy

**Lazy-fetch behavior for channel names:**
1. Check existing cache (complete or partial)
2. If found, return immediately
3. If not found, fetch more pages from API until found or exhausted
4. Save progress to cache after each page (resume-friendly)

This means:
- First lookup of a new channel name may be slow (fetches pages)
- Subsequent lookups are instant (cached)
- Cache grows organically based on usage
- Direct channel IDs always work without any API calls

**Pre-warming the cache (optional):**
```bash
# Fetch a few pages to cache common channels
slack-agent-cli cache populate channels
slack-agent-cli cache populate channels
slack-agent-cli cache populate channels
```

### 4.5 Config Schema

```json
{
  "version": 1,
  "user_token": "xoxp-123...",
  
  "defaults": {
    "output_format": "human",
    "include_bots": false,
    "text_chunk_limit": 4000
  },
  
  "channels": {
    "C123ABC": {
      "name": "general",
      "require_mention": true,
      "allowed_users": ["U456DEF"]
    }
  }
}
```

### 4.6 Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `SLACK_USER_TOKEN` | Override user token |
| `SLACK_CLI_CONFIG` | Config file path |
| `SLACK_CLI_FORMAT` | Default output format (`json` or `human`) |

---

## 5. Output Formats

### 5.1 Human-Readable (Default)

Designed for quick visual inspection:

```
$ slack-agent-cli messages list --channel "#general" --limit 3

#general - Last 3 messages
──────────────────────────────────────────────────
[10:32:45] @alice:
  Hello everyone!

[10:33:01] @bob:
  Hey Alice! How's the project going?

[10:33:15] @alice (in thread):
  Making good progress, will share an update soon.
──────────────────────────────────────────────────
```

### 5.2 JSON (Machine-Readable)

For agent parsing:

```bash
$ slack-agent-cli messages list --channel "#general" --limit 3 --json
```

```json
{
  "ok": true,
  "channel": {
    "id": "C123ABC",
    "name": "general"
  },
  "messages": [
    {
      "ts": "1705312365.000100",
      "user": "U456DEF",
      "username": "alice",
      "text": "Hello everyone!",
      "thread_ts": null,
      "reply_count": 1,
      "reactions": [{"name": "wave", "count": 2}]
    },
    {
      "ts": "1705312381.000200",
      "user": "U789GHI",
      "username": "bob",
      "text": "Hey Alice! How's the project going?",
      "thread_ts": null,
      "reply_count": 0,
      "reactions": []
    },
    {
      "ts": "1705312395.000300",
      "user": "U456DEF",
      "username": "alice",
      "text": "Making good progress, will share an update soon.",
      "thread_ts": "1705312365.000100",
      "reply_count": 0,
      "reactions": []
    }
  ],
  "has_more": true,
  "response_metadata": {
    "next_cursor": "dXNlcl9..."
  }
}
```

---

## 6. Agent Integration Examples

### 6.1 Claude Code / OpenCode Tool Definition

```markdown
## slack-agent-cli

A command-line tool for interacting with Slack as yourself.

### Reading message history
```bash
# Get last 20 messages from a channel
slack-agent-cli messages list --channel "#general" --limit 20 --json
```

### Searching messages
```bash
# Search for specific content
slack-agent-cli messages search --query "deployment failed" --json
```

### Sending messages
```bash
# Send a message
slack-agent-cli messages send --channel "#general" --text "Hello!"

# Reply in a thread
slack-agent-cli messages send --channel "#general" --thread "1705312365.000100" --text "Reply"
```

### Reacting to messages
```bash
slack-agent-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"
```
```

### 6.2 Example Agent Workflow

```bash
# Agent wants to check #support and respond to questions

# 1. Check recent messages
slack-agent-cli messages list --channel "#support" --since 1h --json | jq '.messages[]'

# 2. Search for specific issues
slack-agent-cli messages search --query "error in:#support" --json

# 3. Send a response
slack-agent-cli messages send --channel "#support" --thread "$THREAD_TS" --text "Here's the answer..."

# 4. Add acknowledgment reaction
slack-agent-cli reactions add --channel "#support" --ts "$MESSAGE_TS" --emoji "white_check_mark"
```

### 6.3 Using Channel IDs for Speed

For maximum speed, use channel IDs directly (no cache lookup needed):

```bash
# Direct channel ID - always instant
slack-agent-cli messages list --channel C074S0L3MCG --limit 20

# Channel name - may fetch pages on first use, then cached
slack-agent-cli messages list --channel "#support-bot-testing" --limit 20
```

---

## 7. Required Slack App Permissions

### 7.1 User Token Scopes

Configure these scopes in your Slack App under **OAuth & Permissions → User Token Scopes**.

**Minimum Required Scopes:**

| Scope | Purpose | Commands Enabled |
|-------|---------|------------------|
| `channels:read` | List public channels user is member of | `channels list` |
| `search:read` | Search messages across workspace | `messages search` |
| `identify` | Verify user identity | `auth test`, `auth whoami` |

**Additional Scopes for Full Functionality:**

| Scope | Purpose | Commands Enabled |
|-------|---------|------------------|
| `channels:history` | Read messages from public channels | `messages list` |
| `groups:read` | List private channels user is member of | `channels list --types private_channel` |
| `groups:history` | Read messages from private channels | `messages list` (private channels) |
| `im:read` | List direct messages | `channels list --types im` |
| `im:history` | Read direct messages | `messages list` (DMs) |
| `mpim:read` | List group direct messages | `channels list --types mpim` |
| `mpim:history` | Read group DMs | `messages list` (group DMs) |
| `chat:write` | Send messages as yourself | `messages send`, `messages reply` |
| `users:read` | List workspace members | `users list`, `users info` |
| `reactions:read` | Read reactions on messages | `reactions list` |
| `reactions:write` | Add/remove reactions | `reactions add`, `reactions remove` |
| `pins:read` | Read pinned messages | `pins list` |
| `pins:write` | Pin/unpin messages | `pins add`, `pins remove` |
| `files:read` | Read file info | `files list`, `files info` |
| `files:write` | Upload files | `files upload` |
| `emoji:read` | List custom emoji | `emoji list` |

**Note:** After adding scopes, you must **reinstall the app** to your workspace to get a new token with the updated permissions.

---

## 8. Error Handling

### 8.1 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error (missing config, invalid tokens) |
| 3 | Authentication error (invalid/expired tokens) |
| 4 | Rate limit exceeded |
| 5 | Network error |
| 6 | Permission denied (missing scopes) |
| 7 | Resource not found (channel, user, message) |

### 8.2 Error Output Format

**Human-readable:**
```
Error: Channel not found: #nonexistent
```

**JSON:**
```json
{
  "ok": false,
  "error": {
    "code": "channel_not_found",
    "message": "Channel not found: #nonexistent"
  }
}
```

---

## 9. Implementation Plan

### Phase 1: Core Infrastructure
- [x] Project setup (Go)
- [x] Config management (load/save/validate)
- [x] Slack SDK integration
- [x] Auth test command

### Phase 2: Read Operations
- [x] `channels list`
- [x] `messages list`
- [x] `users list/info`
- [ ] `reactions list`

### Phase 3: Write Operations
- [x] `messages send`
- [x] `messages edit/delete`
- [x] `reactions add/remove`
- [x] `pins add/remove/list`

### Phase 4: Search & Advanced
- [x] `messages search`
- [ ] `files upload/download`
- [ ] `channels join/leave`

---

## 10. Technology Choices

### Go ✓ (Implemented)

**Pros:**
- Single binary distribution
- Excellent CLI libraries (cobra, viper)
- Fast startup time
- Good Slack SDK (slack-go/slack)

The CLI is built in Go using slack-go/slack. Single binary distribution and fast startup are achieved.

---

## 11. Security Considerations

1. **Token Storage**: Config file should have 600 permissions
2. **Token Validation**: Validate tokens on startup, fail fast
3. **Rate Limiting**: Respect Slack's rate limits, implement backoff
4. **Audit Logging**: Optional `--verbose` flag for debugging
5. **No Token Echo**: Never print tokens in output

---

## 12. Future Considerations

### Considered for Future
- Multi-workspace support via named profiles
- Plugin system for custom commands
- Integration with other messaging platforms (Discord, Teams)

### Not Planned (Not for Now)
- **Socket Mode / Real-time Event Streaming** - The CLI is designed for batch operations and polling. Real-time streaming via Socket Mode adds complexity without clear benefit for the target use cases (scripts, cron jobs, AI agents). For real-time needs, use the Slack API directly or webhooks.
- **Interactive Watch Command** - Long-running processes conflict with the stateless, pipe-first design philosophy.
