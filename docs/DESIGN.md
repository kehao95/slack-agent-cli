# Slack CLI Design Document

## 1. Executive Summary

A command-line interface for AI coding agents to interact with Slack workspaces. The CLI exposes all Slack operations as shell commands, enabling agents like Claude Code and OpenCode to read messages, send replies, manage reactions, and more.

**Key Design Decisions:**
- **Socket Mode** for real-time message streaming
- **Hybrid watch model**: `watch` for real-time, `messages list` for batch/history
- **Single workspace** per configuration
- **Config file** for auth storage (`~/.config/slack-cli/config.json`)
- **Both output formats**: Human-readable default, `--json` flag for structured output

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Coding Agent                          │
│                  (Claude Code / OpenCode)                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Subprocess / Shell exec
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         slack-cli                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │  watch   │  │ messages │  │  send    │  │  reactions   │   │
│  │ (stream) │  │  (batch) │  │          │  │              │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘   │
│        │              │            │              │             │
│        └──────────────┴────────────┴──────────────┘             │
│                              │                                  │
│                    ┌─────────▼─────────┐                       │
│                    │   Slack SDK       │                       │
│                    │  (@slack/bolt)    │                       │
│                    └─────────┬─────────┘                       │
└──────────────────────────────│──────────────────────────────────┘
                               │
                               │ Socket Mode / Web API
                               ▼
                    ┌─────────────────────┐
                    │    Slack Platform   │
                    └─────────────────────┘
```

---

## 3. Command Structure

### 3.1 Top-Level Commands

```
slack-cli
├── config          # Configuration management
│   ├── init        # Interactive setup wizard
│   ├── show        # Display current config
│   └── set         # Set config values
│
├── auth            # Authentication
│   ├── test        # Verify credentials work
│   └── whoami      # Show current bot/user info
│
├── watch           # Real-time message streaming (Socket Mode)
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
│   └── search      # Search messages (requires user token)
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

#### `slack-cli config init`

Interactive setup wizard to configure the CLI.

```bash
$ slack-cli config init

Slack CLI Configuration
=======================

1. Create a Slack app at https://api.slack.com/apps
2. Enable Socket Mode and create an App Token (xapp-...)
3. Add Bot Token Scopes and install to workspace
4. Copy the Bot Token (xoxb-...)

? App Token (xapp-...): xapp-1-A123-456-abc...
? Bot Token (xoxb-...): xoxb-123-456-abc...
? User Token (xoxp-..., optional): 

Testing connection... ✓
Bot name: my-bot
Workspace: My Workspace

Configuration saved to ~/.config/slack-cli/config.json
```

---

#### `slack-cli watch`

Real-time message streaming via Socket Mode.

```bash
slack-cli watch [options]

Options:
  --channels <list>      Comma-separated channel names/IDs to watch (default: all)
  --include-bots         Include messages from bots (default: false)
  --include-own          Include bot's own messages (default: false)
  --include-threads      Include thread replies (default: true)
  --timeout <duration>   Exit after duration (e.g., "60s", "5m", "1h")
  --events <list>        Event types to watch (default: message,reaction_added,reaction_removed)
  --json                 Output as JSON lines (one event per line)
  --quiet                Suppress connection status messages
```

**Output (Human-readable):**
```
[2024-01-15 10:32:45] #general | @alice: Hello everyone!
[2024-01-15 10:32:48] #general | @bob reacted with :wave: to "Hello everyone!"
[2024-01-15 10:33:01] #general | @alice (thread): Let's discuss the new feature
```

**Output (JSON):**
```json
{"type":"message","ts":"1705312365.000100","channel":"C123ABC","channel_name":"general","user":"U456DEF","username":"alice","text":"Hello everyone!","thread_ts":null}
{"type":"reaction_added","ts":"1705312368.000200","channel":"C123ABC","user":"U789GHI","username":"bob","reaction":"wave","item_ts":"1705312365.000100"}
{"type":"message","ts":"1705312381.000300","channel":"C123ABC","channel_name":"general","user":"U456DEF","username":"alice","text":"Let's discuss the new feature","thread_ts":"1705312365.000100"}
```

---

#### `slack-cli messages list`

Fetch message history using Slack's conversations.history API.

```bash
slack-cli messages list [options]

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
slack-cli messages list --channel "#general" --limit 20

# Get messages from the last hour
slack-cli messages list --channel "#general" --since 1h --json

# Get thread replies
slack-cli messages list --channel "#general" --thread "1705312365.000100"
```

After the first invocation warms the cache, subsequent `messages list` commands reuse the stored channel and user maps so resolution becomes effectively instantaneous unless `--refresh-cache` is specified.

---

#### `slack-cli messages send`

Send a message to a channel or user.

```bash
slack-cli messages send [options]

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
slack-cli messages send --channel "#general" --text "Hello from CLI!"

# Reply in thread
slack-cli messages send --channel "#general" --thread "1705312365.000100" --text "Thread reply"

# Pipe message content
echo "Multi-line\nmessage" | slack-cli messages send --channel "#general"

# Send to user DM
slack-cli messages send --channel "@alice" --text "Private message"
```

---

#### `slack-cli reactions add/remove/list`

Manage emoji reactions.

```bash
# Add reaction
slack-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# Remove reaction
slack-cli reactions remove --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# List reactions on a message
slack-cli reactions list --channel "#general" --ts "1705312365.000100" --json
```

---

## 4. Configuration

### 4.1 Workspace Context

The production bot currently runs in `support-bot-testing (C074S0L3MCG)`. Examples throughout this document assume that workspace unless otherwise stated.

### 4.2 Config File Location

```
~/.config/slack-cli/config.json
```

Or via `SLACK_CLI_CONFIG` environment variable.

### 4.3 Persistent Cache Location

```
~/.config/slack-cli/cache/
```

- Separate JSON files per domain (e.g., `channels.json`, `users.json`).
- Stored alongside the config directory with directories created at `0700` and files at `0600` permissions.
- Each file contains a payload of Slack metadata plus a `fetched_at` ISO 8601 timestamp used for TTL checks.
- Cache entries default to a 7-day TTL and are refreshed automatically when stale or when commands are invoked with `--refresh-cache`.
- Any command that mutates Slack state (e.g., channel creation) must invalidate affected cache files to prevent stale reads.
- Goal: after the first metadata fetch, commands like `messages list` in `support-bot-testing (C074S0L3MCG)` or any other workspace replay immediately by using cached channel/user lookups.

### 4.4 Config Schema

```json
{
  "version": 1,
  "app_token": "xapp-1-A123...",
  "bot_token": "xoxb-123...",
  "user_token": "xoxp-123...",
  
  "defaults": {
    "output_format": "human",
    "include_bots": false,
    "text_chunk_limit": 4000
  },
  
  "watch": {
    "channels": [],
    "events": ["message", "reaction_added", "reaction_removed"],
    "include_threads": true
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

### 4.5 Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `SLACK_APP_TOKEN` | Override app token |
| `SLACK_BOT_TOKEN` | Override bot token |
| `SLACK_USER_TOKEN` | Override user token |
| `SLACK_CLI_CONFIG` | Config file path |
| `SLACK_CLI_FORMAT` | Default output format (`json` or `human`) |

---

## 5. Output Formats

### 5.1 Human-Readable (Default)

Designed for quick visual inspection:

```
$ slack-cli messages list --channel "#general" --limit 3

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
$ slack-cli messages list --channel "#general" --limit 3 --json
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

### 5.3 JSON Lines (for `watch` streaming)

Each event is a single JSON line for easy parsing:

```jsonl
{"type":"message","ts":"1705312365.000100","channel":"C123ABC","user":"U456DEF","text":"Hello!"}
{"type":"reaction_added","ts":"1705312368.000200","channel":"C123ABC","reaction":"wave","item_ts":"1705312365.000100"}
```

---

## 6. Agent Integration Examples

### 6.1 Claude Code / OpenCode Tool Definition

```markdown
## slack-cli

A command-line tool for interacting with Slack.

### Watching for messages
```bash
# Watch all channels for 60 seconds, output as JSON
slack-cli watch --timeout 60s --json
```

### Reading message history
```bash
# Get last 20 messages from a channel
slack-cli messages list --channel "#general" --limit 20 --json
```

### Sending messages
```bash
# Send a message
slack-cli messages send --channel "#general" --text "Hello!"

# Reply in a thread
slack-cli messages send --channel "#general" --thread "1705312365.000100" --text "Reply"
```

### Reacting to messages
```bash
slack-cli reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"
```
```

### 6.2 Example Agent Workflow

```bash
# Agent wants to monitor #support and respond to questions

# 1. Start watching (agent runs this in background or with timeout)
slack-cli watch --channels "#support" --timeout 5m --json > /tmp/slack-events.jsonl &

# 2. Process incoming messages
tail -f /tmp/slack-events.jsonl | while read -r event; do
  # Agent processes each event...
done

# 3. Or use batch mode for one-shot queries
slack-cli messages list --channel "#support" --since 1h --json | jq '.messages[]'

# 4. Send a response
slack-cli messages send --channel "#support" --thread "$THREAD_TS" --text "Here's the answer..."

# 5. Add acknowledgment reaction
slack-cli reactions add --channel "#support" --ts "$MESSAGE_TS" --emoji "white_check_mark"
```

---

## 7. Feature Mapping (vs Molt Bot)

| Molt Bot Feature | slack-cli Equivalent |
|------------------|---------------------|
| Socket Mode connection | `watch` command |
| HTTP Mode (webhooks) | Not supported (Socket Mode only) |
| Message history context | `messages list --since` |
| Send/edit/delete messages | `messages send/edit/delete` |
| Reactions | `reactions add/remove/list` |
| Pins | `pins add/remove/list` |
| Member info | `users info` |
| Custom emoji list | `emoji list` |
| File uploads | `files upload` |
| Slash commands | Not supported (agent-driven, not bot-driven) |
| Multi-account | Not supported (single workspace) |
| Reply threading modes | Manual via `--thread` flag |
| DM handling | `--channel "@username"` |

---

## 8. Required Slack App Permissions

### 8.1 Bot Token Scopes (Required)

| Scope | Purpose |
|-------|---------|
| `channels:history` | Read messages from public channels |
| `channels:read` | List and get info about public channels |
| `chat:write` | Send messages |
| `groups:history` | Read messages from private channels |
| `groups:read` | List and get info about private channels |
| `im:history` | Read direct messages |
| `im:read` | List and get info about DMs |
| `im:write` | Start DMs |
| `mpim:history` | Read group DMs |
| `mpim:read` | List and get info about group DMs |
| `reactions:read` | Read reactions |
| `reactions:write` | Add/remove reactions |
| `users:read` | Read user info |
| `pins:read` | Read pinned messages |
| `pins:write` | Pin/unpin messages |
| `files:read` | Read file info |
| `files:write` | Upload files |
| `emoji:read` | List custom emoji |

### 8.2 User Token Scopes (Optional)

| Scope | Purpose |
|-------|---------|
| `search:read` | Search messages |

### 8.3 App Token Scopes (Required for Socket Mode)

| Scope | Purpose |
|-------|---------|
| `connections:write` | Establish Socket Mode connection |

---

## 9. Event Subscriptions (for `watch`)

| Event | Description |
|-------|-------------|
| `message.channels` | Messages in public channels |
| `message.groups` | Messages in private channels |
| `message.im` | Direct messages |
| `message.mpim` | Group DMs |
| `reaction_added` | Reaction added to message |
| `reaction_removed` | Reaction removed |
| `member_joined_channel` | User joined channel |
| `member_left_channel` | User left channel |
| `channel_rename` | Channel renamed |
| `pin_added` | Message pinned |
| `pin_removed` | Message unpinned |
| `app_mention` | Bot was @mentioned |

---

## 10. Error Handling

### 10.1 Exit Codes

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

### 10.2 Error Output Format

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

## 11. Implementation Plan

### Phase 1: Core Infrastructure
- [ ] Project setup (Go or Node.js)
- [ ] Config management (load/save/validate)
- [ ] Slack SDK integration
- [ ] Auth test command

### Phase 2: Read Operations
- [ ] `channels list`
- [ ] `messages list`
- [ ] `users list/info`
- [ ] `reactions list`

### Phase 3: Write Operations
- [ ] `messages send`
- [ ] `messages reply`
- [ ] `reactions add/remove`
- [ ] `pins add/remove`

### Phase 4: Real-time
- [ ] `watch` command (Socket Mode)
- [ ] Event filtering
- [ ] Timeout handling

### Phase 5: Advanced Features
- [ ] `messages edit/delete`
- [ ] `messages search` (user token)
- [ ] `files upload/download`
- [ ] `channels join/leave`

---

## 12. Technology Choices

### Option A: Go

**Pros:**
- Single binary distribution
- Excellent CLI libraries (cobra, viper)
- Fast startup time
- Good Slack SDK (slack-go/slack)

**Cons:**
- More verbose code

### Option B: Node.js/TypeScript

**Pros:**
- Official Slack Bolt SDK
- Faster development
- Rich ecosystem

**Cons:**
- Requires Node.js runtime
- Slower startup

### Recommendation: **Go**

For a CLI tool used by agents, single binary distribution and fast startup are critical. The slack-go library provides full API coverage.

---

## 13. Security Considerations

1. **Token Storage**: Config file should have 600 permissions
2. **Token Validation**: Validate tokens on startup, fail fast
3. **Rate Limiting**: Respect Slack's rate limits, implement backoff
4. **Audit Logging**: Optional `--verbose` flag for debugging
5. **No Token Echo**: Never print tokens in output

---

## 14. Future Considerations

- Multi-workspace support via named profiles
- Plugin system for custom commands
- Caching layer for frequently accessed data
- Integration with other messaging platforms (Discord, Teams)
