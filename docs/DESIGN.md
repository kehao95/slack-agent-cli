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
- **Config file** for auth storage (`~/.config/slack-cli/config.json`)
- **JSON default output**, `--human/-H` flag for human-readable tables

**Agent boundary contract:** [`READ_ONLY.md`](READ_ONLY.md) specifies the
environment-enforced remote read-only policy, reviewed method allowlist, local
operation exceptions, ordinary search identity requirements, and email
availability. [`IDENTITY.md`](IDENTITY.md) defines the global ID / `@username` /
display-name contract. User Action Token contextual search is deferred to
[issue #5](https://github.com/kehao95/slack-agent-cli/issues/5).

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
│                         slk                               │
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
slk
├── api <method>     # Generic Slack Web API escape hatch
│
├── auth            # Authentication
│   ├── login       # Save a token
│   ├── oauth       # Run the OAuth callback flow
│   ├── test        # Verify credentials work
│   └── whoami      # Show current user info
│
├── cache           # Cache management (for name resolution)
│   ├── populate    # Fetch and cache channels/users
│   ├── status      # Show cache state
│   └── clear       # Clear cached data
│
├── lists           # Slack List operations
│   ├── items       # Fetch rows/items from a Slack List
│   └── item        # Fetch one row/item with list metadata
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
│   ├── lookup      # Look up a user by email
│   ├── profile     # Get profile details
│   ├── status      # Get/set/clear custom status
│   ├── conversations # List a user's conversations
│   └── presence    # Check user presence
│
├── files           # File operations
│   ├── upload      # Upload a file
│   ├── download    # Download a file
│   ├── list        # List files with cursor pagination
│   ├── info        # Get file metadata
│   ├── delete      # Delete a file
│   ├── share-public # Create a public URL
│   └── revoke-public # Revoke a public URL
│
├── search          # Unified search
│   ├── all         # Search messages and files
│   ├── messages    # Search messages
│   └── files       # Search files
│
├── usergroups      # User group operations
│   ├── list        # List user groups
│   ├── members     # List/replace members
│   ├── create      # Create a group
│   ├── update      # Update a group
│   ├── enable      # Enable a group
│   └── disable     # Disable a group
│
└── emoji           # Emoji operations
    └── list        # List custom emoji
```

---

### 3.2 Command Details

All dedicated user-reference parameters and normalized user output follow the
[global identity contract](IDENTITY.md). Only canonical IDs, complete `<@ID>`
mentions, and explicit `@username` handles are accepted. Display names are
presentation only; bare handles and implicit email aliases are rejected.
Normalized `user` fields stay IDs, with separate `username` (`@handle`) and
`display_name` metadata. Native raw payloads retain Slack's field meanings.

#### `slk api <family.method>`

Call any Slack Web API method without waiting for an SDK or dedicated CLI
wrapper. The input contract is one JSON object, supplied inline, as `@file`, or
from stdin using `--data -`. Authentication follows the active configured role.

```bash
slk api conversations.info --data '{"channel":"C123"}'
slk api conversations.list --data '{"limit":200}' --all
```

Without `--all`, the complete Slack response is passed through as JSON. With
`--all`, the CLI follows `response_metadata.next_cursor` and wraps the complete
responses in a `pages` array. HTTP 429 retries honor `Retry-After`; pagination
can be paced with `--page-delay`.

#### Resource commands added in P1

The dedicated resource layer favors stable agent workflows over a one-command-
per-method mapping:

```bash
# Files use cursor pagination and refuse to overwrite downloads unless requested.
slk files list --channel "#general" --all
slk files download --file F123 --output ./artifact.bin

# Search uses Slack's page pagination. The legacy messages search command remains valid.
slk search all --query "deployment" --all
slk search files --query "quarterly report" --page 2

# Empty --user means the authenticated user for profile/status/conversation calls.
slk users lookup --email alice@example.com
slk users status set --text "Focus" --emoji :headphones: --expires-in 2h

# User groups accept IDs, @handles, or names; members accept IDs or @usernames.
slk usergroups members set --group @oncall --members @alice,@bob
```

Specialized commands emit normalized result envelopes and support `--human`.
`slk api` remains the escape hatch for uncommon parameters and APIs that do not
yet have a stable resource abstraction.

#### Authentication setup

The CLI is non-interactive. Save a user token directly, use environment
variables, or run the OAuth callback flow:

```bash
slk auth login --token xoxp-your-token --verify
export SLACK_USER_TOKEN='xoxp-your-token'
slk auth oauth --client-id "$SLACK_CLIENT_ID" --client-secret "$SLACK_CLIENT_SECRET"
```

OAuth saves both returned user and bot credentials without exposing either in
the browser or command output. The active identity is selected with
`SLACK_CLI_ROLE=user|bot` (or the config `role` field).

---

#### `slk messages list`

Fetch message history using Slack's conversations.history API.

```bash
slk messages list [options]

Options:
  --channel <name|id>    Channel to fetch from (required)
  --limit <n>            Maximum messages per Slack page (default: 50)
  --cursor <cursor>      Continue history or thread pagination
  --all                  Follow every continuation cursor
  --page-delay <dur>     Optional delay between pages
  --retry-rate-limit     Honor Slack Retry-After responses (default: true)
  --max-retries <n>      Maximum rate-limit retries per page (default: 3)
  --since <time>         Messages after this time (ISO 8601 or relative: "1h", "2d")
  --until <time>         Messages before this time
  --thread <ts>          Fetch replies in a specific thread
  --include-metadata     Request full metadata for each returned message
  --include-bots         Include bot messages (default: true)
  --refresh-cache        Force refresh of cached channel/user metadata before running
  --resolved-json        Enrich JSON with channel names and separate user metadata (default: true)
  --raw-json             Preserve raw Slack IDs in JSON output
  --json                 Output as JSON
```

**Example:**
```bash
# Get last 20 messages from #general
slk messages list --channel "#general" --limit 20

# Get messages from the last hour
slk messages list --channel "#general" --since 1h --json

# Get thread replies
slk messages list --channel "#general" --thread "1705312365.000100"

# Fetch every thread page while honoring Slack rate limits
slk messages list --channel "#general" --thread "1705312365.000100" --all

# Return opaque Slack metadata when it is present on messages
slk messages list --channel "#general" --include-metadata --raw-json
```

After the first invocation warms the cache, subsequent `messages list` commands reuse the stored channel and user maps so resolution becomes effectively instantaneous unless `--refresh-cache` is specified.

JSON output adds presentation metadata without replacing user IDs: `user` and `user_id` contain the stable ID, `username` contains only a real `@handle`, and `display_name` is presentation only. Nested user references remain IDs. Channel resolution still uses `#channel` with a companion `channel_id`. Use `--raw-json` for native message fields without identity enrichment; raw `username` is not a trusted handle. See [IDENTITY.md](IDENTITY.md).

`--include-metadata` requests Slack's full metadata for every message from both
history and thread-reply requests, including every page followed by `--all`.
When Slack returns it, a message's `metadata` contains `event_type` and an
opaque `event_payload`; the CLI does not resolve, add, or rewrite user-looking
values within that payload. Metadata can be absent, in which case the message
has no `metadata` field. It is distinct from the response-level
`response_metadata`, which carries pagination data such as `next_cursor`.
`--raw-json` preserves the native output shape but does not request full
metadata by itself: pair it with `--include-metadata` when needed. The flag is
read-only, requires no new scopes or credentials, and does not create trace
IDs, generate metadata, or guarantee that an execution trace is available.

---

#### `slk messages send`

Send a message to a channel or user.

```bash
slk messages send [options]

Options:
  --channel <name|id>    Target channel (use @user for DM)
  --text <message>       Message text (can also be piped via stdin)
  --thread <ts>          Reply in thread
  --blocks <json|@file|-> Block Kit JSON (inline, file, or stdin)
  --image <path>         Upload and share a local image
  --alt-text <text>      Accessible alt text for the uploaded image
  --unfurl-links         Unfurl URLs (default: true)
  --unfurl-media         Unfurl media (default: true)
  --json                 Output sent message details as JSON
```

**Examples:**
```bash
# Simple message
slk messages send --channel "#general" --text "Hello from CLI!"

# Reply in thread
slk messages send --channel "#general" --thread "1705312365.000100" --text "Thread reply"

# Pipe message content
echo "Multi-line\nmessage" | slk messages send --channel "#general"

# Send to user DM
slk messages send --channel "@alice" --text "Private message"

# Send an image with an optional caption
slk messages send --channel "#general" --image ./screenshot.png --mrkdwn "Latest screenshot"

# Send an image as a thread reply
slk messages send --channel "#general" --thread "1705312365.000100" --image ./screenshot.png
```

Block objects are structurally checked for a non-empty `type`, then forwarded
without a local type whitelist. This keeps the CLI forward-compatible with
Block Kit additions in Slack that have not reached the pinned Go SDK yet.

#### Extended conversations and messages

`slk conversations` is the canonical resource surface for public channels,
private channels, DMs, and group DMs. It provides `list`, `info`, `create`,
`archive`, `unarchive`, `rename`, `topic`, `purpose`, `members`, `invite`,
`kick`, `open`, `close`, `mark`, `join`, and `leave`. The existing `channels`
commands remain available for compatibility.

Message lifecycle commands also include:

```bash
slk messages permalink --channel '#general' --ts "$TS"
slk messages ephemeral --channel '#general' --user @alice --text 'Visible to you'
slk messages schedule --channel '#general' --post-at 10m --mrkdwn 'Reminder'
slk messages scheduled list --all
slk messages scheduled delete --channel C123 --id Q123
slk messages stream start --channel D123 --recipient-user U123
slk messages stream append --channel D123 --ts "$TS" --mrkdwn - < chunk.md
slk messages stream stop --channel D123 --ts "$TS" --blocks @final-blocks.json
```

#### `slk lists items`

Fetch items from a Slack List via `slackLists.items.list`.

```bash
slk lists items [options]

Options:
  --list <id|url>       Slack List ID or URL (required)
  --limit <n>           Max items to return (default: 100)
  --cursor <cursor>     Pagination cursor
  --all                 Fetch all pages of items
  --archived            Return archived items instead of active items
  --json                Output as JSON
```

**Examples:**
```bash
# Read items by ID
slk lists items --list F0BFMJY6ZTQ

# Read items by Slack URL
slk lists items --list https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ

# Read all pages
slk lists items --list F0BFMJY6ZTQ --all
```

#### `slk lists item`

Fetch one item from a Slack List via `slackLists.items.info`.

```bash
slk lists item [options]

Options:
  --list <id|url>                 Slack List ID or URL (required)
  --id <record-id>                Slack List record/item ID (required)
  --include-is-subscribed         Include subscription state when Slack provides it
  --json                          Output as JSON
```

**Examples:**
```bash
# Read one item by record id
slk lists item --list F0BFMJY6ZTQ --id Rec018B8RR603

# Include subscription state
slk lists item --list F0BFMJY6ZTQ --id Rec018B8RR603 --include-is-subscribed
```

When `--image` is used, the command uploads the local file and shares it in
the target channel using Slack's external upload sequence. It may be used by
itself, with one text input as a caption, or with `--blocks`. JSON output
contains `file_id`, `filename`, `channel`, and optional `thread_ts`; Slack does
not return a message timestamp from the external upload completion endpoint.

---

#### `slk messages search`

Search messages across the workspace.

```bash
slk messages search [options]

Options:
  --query <text>         Search query (required)
  --limit <n>            Max results to return (default: 20)
  --sort <field>         Sort by 'score' or 'timestamp' (default: timestamp)
  --sort-dir <dir>       Sort direction 'asc' or 'desc' (default: desc)
  --resolved-json        Enrich JSON with channel names and separate user metadata (default: true)
  --raw-json             Preserve raw Slack IDs in JSON output
  --json                 Output as JSON
```

**Examples:**
```bash
# Basic search
slk messages search --query "deployment failed"

# Search with advanced syntax
slk messages search --query "from:@alice in:#general"

# Search and sort by relevance
slk messages search --query "error" --sort score --limit 20
```

---

#### `slk reactions add/remove/list`

Manage emoji reactions.

```bash
# Add reaction
slk reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# Remove reaction
slk reactions remove --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"

# List reactions on a message
slk reactions list --channel "#general" --ts "1705312365.000100" --json
```

---

## 4. Configuration

### 4.1 Config File Location

```
~/.config/slack-cli/config.json
```

Or via `SLACK_CLI_CONFIG` environment variable.

### 4.2 Persistent Cache Location

```
~/.config/slack-cli/cache/
```

- Separate JSON files per domain (for example `channels.json`).
- User lookups use per-user read-through cache entries instead of a workspace-wide user snapshot.
- Stored alongside the config directory with directories created at `0700` and files at `0600` permissions.
- Each file contains a payload of Slack metadata plus a `fetched_at` ISO 8601 timestamp used for TTL checks.
- Cache entries default to a 7-day TTL and are refreshed automatically when stale or when commands are invoked with `--refresh-cache`.
- Any command that mutates Slack state (e.g., channel creation) must invalidate affected cache files to prevent stale reads.

### 4.3 Cache Population Commands

The `cache` command group provides explicit control over metadata caching with incremental pagination support. This design allows AI agents to:
1. Control when API calls are made (no surprise rate limits)
2. Resume interrupted fetches without losing progress
3. Monitor cache state before running other commands

#### `slk cache populate`

Fetch and cache channels from Slack with incremental pagination.

```bash
slk cache populate <channels> [options]

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
slk cache populate channels
slk cache populate channels  # Continues from cursor
slk cache populate channels  # Continues until done

# Or fetch all at once with rate limiting
slk cache populate channels --all --page-delay 2s

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

#### `slk cache status`

Show current cache state.

```bash
slk cache status [options]

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

#### `slk cache clear`

Clear cached data.

```bash
slk cache clear [channels|users]

# Clear all caches
slk cache clear

# Clear specific cache
slk cache clear channels
slk cache clear users
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
slk cache populate channels
slk cache populate channels
slk cache populate channels
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
$ slk messages list --channel "#general" --limit 3

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
$ slk messages list --channel "#general" --limit 3 --json
```

```json
{
  "channel": "#general",
  "channel_id": "C123ABC",
  "channel_name": "general",
  "messages": [
    {
      "ts": "1705312365.000100",
      "user": "U456DEF",
      "user_id": "U456DEF",
      "username": "@alice",
      "display_name": "Alice Example",
      "text": "Hello everyone!",
      "thread_ts": null,
      "reply_count": 1,
      "reactions": [{"name": "wave", "count": 2, "users": ["U456DEF"], "user_ids": ["U456DEF"]}]
    },
    {
      "ts": "1705312381.000200",
      "user": "U789GHI",
      "user_id": "U789GHI",
      "username": "@bob",
      "display_name": "Bob Example",
      "text": "Hey Alice! How's the project going?",
      "thread_ts": null,
      "reply_count": 0,
      "reactions": []
    },
    {
      "ts": "1705312395.000300",
      "user": "U456DEF",
      "user_id": "U456DEF",
      "username": "@alice",
      "display_name": "Alice Example",
      "text": "Making good progress, will share an update soon.",
      "thread_ts": "1705312365.000100",
      "reply_count": 0,
      "reactions": []
    }
  ],
  "has_more": true,
  "next_cursor": "dXNlcl9..."
}
```

---

## 6. Agent Integration Examples

### 6.1 Claude Code / OpenCode Tool Definition

```markdown
## slk

A command-line tool for interacting with Slack as yourself.

### Reading message history
```bash
# Get last 20 messages from a channel
slk messages list --channel "#general" --limit 20 --json
```

### Searching messages
```bash
# Search for specific content
slk messages search --query "deployment failed" --json
```

### Sending messages
```bash
# Send a message
slk messages send --channel "#general" --text "Hello!"

# Reply in a thread
slk messages send --channel "#general" --thread "1705312365.000100" --text "Reply"
```

### Reacting to messages
```bash
slk reactions add --channel "#general" --ts "1705312365.000100" --emoji "thumbsup"
```
```

### 6.2 Example Agent Workflow

```bash
# Agent wants to check #support and respond to questions

# 1. Check recent messages
slk messages list --channel "#support" --since 1h --json | jq '.messages[]'

# 2. Search for specific issues
slk messages search --query "error in:#support" --json

# 3. Send a response
slk messages send --channel "#support" --thread "$THREAD_TS" --text "Here's the answer..."

# 4. Add acknowledgment reaction
slk reactions add --channel "#support" --ts "$MESSAGE_TS" --emoji "white_check_mark"
```

### 6.3 Using Channel IDs for Speed

For maximum speed, use channel IDs directly (no cache lookup needed):

```bash
# Direct channel ID - always instant
slk messages list --channel C074S0L3MCG --limit 20

# Channel name - may fetch pages on first use, then cached
slk messages list --channel "#support-bot-testing" --limit 20
```

---

## 7. Required Slack App Permissions

### 7.1 User Token Scopes

Configure these scopes in your Slack App under **OAuth & Permissions → User Token Scopes**.

**Minimum Required Scopes:**

| Scope | Purpose | Commands Enabled |
|-------|---------|------------------|
| `channels:read` | List public channels visible to the user | `channels list` |
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
| `channels:write` | Manage public-channel membership and metadata | `conversations create/rename/topic/purpose/join/leave` |
| `groups:write` | Manage private channels | `conversations create/archive/invite/kick/...` |
| `im:write`, `mpim:write` | Open and manage direct conversations | `conversations open/close` |
| `chat:write` | Send messages as yourself | `messages send`, `messages reply` |
| `users:read` | List workspace members | `users list`, `users info` |
| `users:read.email` | Look up users by email | `users lookup` |
| `users.profile:read` | Read full user profiles | `users profile` |
| `users.profile:write` | Update the authenticated user's profile/status | `users status set/clear` |
| `usergroups:read` | Read user groups and membership | `usergroups list/members` |
| `usergroups:write` | Manage user groups and membership | `usergroups create/update/enable/disable/members set` |
| `reactions:read` | Read reactions on messages | `reactions list` |
| `reactions:write` | Add/remove reactions | `reactions add`, `reactions remove` |
| `pins:read` | Read pinned messages | `pins list` |
| `pins:write` | Pin/unpin messages | `pins add`, `pins remove` |
| `files:read` | Read file info and content | `files list/info/download`, `search files` |
| `files:write` | Upload, delete, and change public sharing | `files upload/delete/share-public/revoke-public` |
| `lists:read` | Read Slack Lists | `lists items/item` |
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
- [x] `reactions list`

### Phase 3: Write Operations
- [x] `messages send`
- [x] `messages edit/delete`
- [x] `reactions add/remove`
- [x] `pins add/remove/list`

### Phase 4: Search & Advanced
- [x] `messages search`
- [x] `files upload/download/list/info/delete/share-public/revoke-public`
- [x] `conversations` lifecycle, membership, metadata, and DM operations
- [x] unified `search all/messages/files`
- [x] user lookup/profile/status/conversations and user-group management
- [x] generic `slk api <family.method>` escape hatch

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
