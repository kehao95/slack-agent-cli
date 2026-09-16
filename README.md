# slk

> **Slack for Non-Humans™**

## Why "Non-Humans"?

Humans have eyes, we have `jq`.  
Humans need emojis, we need `thread_ts`.  
Humans want a UI, we just want a clean pipe.  

**This is not a CLI for you. This is a CLI for your digital entities.**

---

> [!WARNING]
> Human, this tool defaults to JSON output. If your biological eyes find this hard to parse, use the `--human` flag. We recommend upgrading your optic nerve or using `jq`.

## Features

- **Pipe-First Design** - Output is always pure JSON (stdout) while logs go to stderr.
- **Agent-Ready** - Stateless authentication, perfect for LLMs, scripts, and cron jobs.
- **Smart Caching** - Resolves channel names (`#general`) to IDs (`C123...`) locally for speed.
- **Read-Only Guardrail** - `SLACK_CLI_READ_ONLY=true` blocks remote mutations and unreviewed API methods before dispatch. See the [boundary and agent recipe](docs/READ_ONLY.md).
- **Image Uploads** - Send local images into channels or threads with Slack's current external upload API.
- **CLI resource expansion** - Canvas and List authoring, channel bookmarks,
  directory search, user settings, message inspection/preview, and file content
  workflows are described in [CLI coverage](docs/CLI_COVERAGE.md).

## Quick Start

### For Digital Entities (Default)

```bash
# Get channel history as minified JSON
slk messages list --channel C12345

# Pipe directly into other tools
slk messages list --channel "#general" | jq '.[].text'

# Ask Slack to include per-message metadata; raw output does not request it alone
slk messages list --channel C12345 --include-metadata --raw-json | jq '.messages[].metadata?'
```

### For Biological Entities

```bash
# Save your token to config
slk auth login --token xoxp-your-token --verify

# List channels in human-readable format
slk channels list --human

# List recent messages
slk messages list --channel "#general" --limit 10 --human
```

### Message Metadata

`slk messages list --include-metadata` requests Slack's full metadata for each
message returned by history or thread-reply listing. A present `metadata` value
has an `event_type` and opaque `event_payload`; values in that payload are left
unchanged. This per-message field is separate from response-level
`response_metadata`, which is used for pagination. Metadata is optional and is
omitted when Slack does not return it. `--raw-json` controls identity enrichment
only, so it must be combined with `--include-metadata` to request full
metadata. The read-only option needs no additional scopes or credentials and
does not create or guarantee an execution trace.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install kehao95/slack-agent-cli/slk
```

### Go Install

```bash
go install github.com/kehao95/slack-agent-cli@latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/kehao95/slack-agent-cli/releases)

## Authentication & Configuration

### How It Works

**This CLI uses User Tokens** - It authenticates as **you** and acts on your behalf in Slack. This means:

- **Acts as you:** Messages, reactions, and actions appear as if you did them
- **Uses your permissions:** Can only access channels/DMs you have access to
- **Flexible setup:** Use direct tokens or the built-in local OAuth callback server
- **Token security:** Keep your token safe - it has the same permissions you do

**User Token** (what this CLI uses):
- Format: `xoxp-...` 
- Represents **you** (the user)
- Perfect for automation, scripts, and AI agents acting on your behalf

**Bot Token** (supported when `SLACK_CLI_ROLE=bot`):
- Format: `xoxb-...`
- Represents a **bot user** (separate identity)
- Requires more setup and different use cases

### Quick Setup (1 minute)

1. **Create Slack App:** Go to https://api.slack.com/apps → **"Create New App"** → **"From an app manifest"**
2. **Choose mode & use manifest:**
   - **Read-Only** (recommended): [`slack-app-manifest-readonly.yaml`](./slack-app-manifest-readonly.yaml)
   - **Full Access:** [`slack-app-manifest-full.yaml`](./slack-app-manifest-full.yaml)
3. **Install to workspace:** Click "Install to Workspace" and authorize
4. **Copy token:** Copy the **User OAuth Token** (starts with `xoxp-`)
5. **Configure:**
   - **Option A (Recommended):** Run `slk auth login --token xoxp-... --verify`
   - **Option B:** Set `export SLACK_USER_TOKEN='xoxp-...'`
   - **Option C:** Use OAuth flow with `slk auth oauth` (see below)

**See [SLACK_SETUP.md](./SLACK_SETUP.md) for detailed setup instructions and mode comparison.**

### OAuth Flow (Alternative)

For automated token exchange, use the built-in OAuth server:

```bash
slk auth oauth --client-id $SLACK_CLIENT_ID --client-secret $SLACK_CLIENT_SECRET
```

This starts a local server on port 8089 with a `/callback` endpoint. Expose it publicly (via your preferred method) and add the callback URL to your Slack app's redirect URIs. The authorization link contains a cryptographically random, single-use `state` value and requests the full manifest's user and bot scopes by default. User and bot tokens returned by Slack are saved to config and are never printed or returned to the browser. Use `--bot-scopes ''` for a user-only install, or `--save=false` only to discard the exchanged credentials.

## Available Commands

```
slk
├── api             # Call any Slack Web API method with JSON
│
├── auth            # Authentication
│   ├── login       # Save token to config
│   ├── oauth       # Start OAuth callback server
│   ├── test        # Verify credentials work
│   └── whoami      # Show current user info
│
├── cache           # Cache management
│   ├── populate    # Fetch and cache channels/users
│   ├── status      # Show cache state
│   └── clear       # Clear cached data
│
├── channels        # Channel operations
│   ├── list        # List accessible channels
│   ├── join        # Join a channel
│   └── leave       # Leave a channel
│
├── conversations   # Unified channel, DM, and group-DM operations
│   ├── list/info/search/create/archive/unarchive
│   ├── rename/topic/purpose
│   ├── members/invite/kick
│   └── open/close/mark/join/leave
│
├── messages        # Message operations
│   ├── list        # Fetch message history
│   ├── get         # Read an exact message URL/ID and optional complete thread
│   ├── preview     # Inspect a message payload locally
│   ├── send        # Send a message
│   ├── edit        # Edit a message
│   ├── delete      # Delete a message
│   ├── permalink   # Get a stable message URL
│   ├── ephemeral   # Send a user-scoped ephemeral message
│   ├── schedule    # Schedule a message
│   ├── scheduled   # List/delete scheduled messages
│   ├── stream      # Start/append/stop agent response streams
│   ├── search      # Search messages
│   └── next        # Wait for the next cached message event
│
├── files           # File operations
│   ├── upload      # Upload and optionally share a local file
│   ├── download    # Download a file by ID
│   ├── read/export # Read text or export content by file ID/URL
│   ├── list        # List files with cursor pagination
│   ├── info        # Inspect file metadata
│   ├── delete      # Delete a file
│   ├── share-public # Create a public URL
│   └── revoke-public # Revoke a public URL
│
├── search          # Unified workspace search
│   ├── all         # Search messages and files
│   ├── messages    # Search messages
│   └── files       # Search files
│
├── events          # Event stream/cache operations
│   ├── stream      # Stream Socket Mode events as NDJSON
│   ├── list        # Query cached daemon events
│   ├── next        # Wait for the next matching cached event
│   ├── claim       # Claim one pending event for processing
│   └── ack         # Acknowledge processed cursor
│
├── daemon          # Local event cache daemon
│   ├── run         # Cache Socket Mode events into SQLite
│   └── status      # Inspect local event cache status
│
├── canvases        # Canvas authoring and content
│   ├── list/create/channel-create/read/export
│   └── edit/delete/sections/share/revoke
│
├── bookmarks       # Channel bookmarks
│   └── list/add/edit/remove
│
├── lists           # Slack List operations
│   ├── items/item  # Fetch items and schema metadata
│   ├── create/update/share/revoke
│   ├── item-add/item-update/item-delete/items-delete
│   └── export/export-start/export-get
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
│   ├── search      # Search the visible user directory
│   ├── info        # Get user details
│   ├── lookup      # Look up a user by email
│   ├── profile     # Get a profile; set selected fields
│   ├── status      # Get, set, or clear custom status
│   ├── conversations # List conversations for a user
│   ├── presence    # Check presence; get/set
│   ├── photo       # Set/delete the authenticated user's photo
│   └── dnd         # Inspect/snooze/end Do Not Disturb
│
├── usergroups      # User group operations
│   ├── list        # List user groups
│   ├── members     # List/set/add/remove members
│   ├── create      # Create a user group
│   ├── update      # Update group metadata
│   ├── enable      # Enable a user group
│   └── disable     # Disable a user group
│
└── emoji           # Emoji operations
    └── list        # List custom emoji
```

## Raw Slack Web API

Use `slk api` as a forward-compatible escape hatch for Slack methods that do
not yet have a resource-oriented command. It uses the active user or bot role,
including the configured cookie for `xoxc-` credentials.

```bash
# Inline JSON
slk api conversations.info --data '{"channel":"C123"}'

# JSON from a file or stdin
slk api chat.postMessage --data @request.json
printf '{"limit":200}' | slk api conversations.list --data -

# Follow response_metadata.next_cursor through every page
slk api conversations.list --data '{"limit":200}' --all
slk api users.list --cursor dXNlcjpVMTIz --all
```

Single-page calls emit Slack's response object unchanged. `--all` emits
`{"ok":true,"page_count":N,"pages":[...]}`, preserving each complete page.
HTTP 429 responses honor Slack's `Retry-After` header and retry up to three
times by default. Use `--max-retries` to change that and `--page-delay` to pace
pagination calls.

## Conversation and Message References

Conversation-taking commands accept `C…`/`G…`/`D…` IDs, `#channel` names,
Slack archive/message URLs, and—where opening a DM is meaningful—`@user`.
User references accept canonical uppercase IDs (`U…`/`W…`), complete `<@ID>`
mentions, or explicit `@username` handles. Handles match Slack's username field
only, case-insensitively. Bare handles, display/real names, implicit email aliases,
and malformed mentions are rejected. Use `users lookup --email` for email lookup.

Normalized user output separates `user_id` (stable ID), `username` (exact
`@handle`), and `display_name` (non-unique presentation only). Scalar `user` and
nested user references stay IDs; resolution only adds metadata. Names never
replace identity, and missing handles are not fabricated from display names.
See the [global identity contract and migration guide](docs/IDENTITY.md) for
batch arguments, raw payloads, events, and an agent recipe.

```bash
slk conversations open --users @alice
slk conversations members --channel '#general' --all
slk messages send --channel @alice --mrkdwn 'Private update'
slk messages list --channel '#general' --all --retry-rate-limit
```

Block Kit flags accept inline JSON, `@path`, or `-` for stdin. The CLI checks
that each block has a `type` and otherwise forwards its JSON unchanged, so new
Slack block types do not need a CLI release.

```bash
slk messages send --channel '#general' --blocks @blocks.json
slk messages schedule --channel '#general' --post-at 10m --mrkdwn 'Reminder'
slk messages stream start --channel D123 --recipient-user U123
slk messages stream append --channel D123 --ts "$TS" --mrkdwn - < chunk.md
slk messages stream stop --channel D123 --ts "$TS"
```

## Use Cases

### Files, Search, and People

```bash
# Upload an agent artifact and share it in a thread
slk files upload --file ./report.pdf --channel "#ops" --thread "$THREAD_TS"

# Search both messages and files, following every page
slk search all --query "incident 142" --all

# Resolve a person without downloading the full user directory
slk users lookup --email alice@example.com

# Replace a user group's membership using agent-friendly references
slk usergroups members set --group @oncall --members @alice,@bob
```

### The "Pipeline" Approach

```bash
# Summarize the last hour of #alerts using an LLM
slk messages list --channel "#alerts" --since 1h | llm "Summarize these alerts"

# Pull the latest helpdesk rows from a Slack List
slk lists items --list https://contentsquare.slack.com/lists/T027K0ZC9/F0BFMJY6ZTQ | jq '.items'

# Fetch the entire list across all pages
slk lists items --list F0BFMJY6ZTQ --all

# Inspect one record with schema-aware field output
slk lists item --list F0BFMJY6ZTQ --id Rec018B8RR603 --human

# Auto-reply to specific errors
slk messages search --query "error: deployment" | \
  jq -r '.matches[].ts' | \
  xargs -I {} slk messages send --channel "#ops" --thread {} --mrkdwn "Investigating..."
```

### Agent Workflow Example

```bash
# 1. Check recent messages
slk messages list --channel "#support" --since 1h

# 2. Search for specific issues
slk messages search --query "error in:#support"

# 3. Send a response
slk messages send --channel "#support" --thread "$THREAD_TS" --mrkdwn "Here's the answer..."

# Send a screenshot with an optional caption
slk messages send --channel "#support" --image ./screenshot.png --mrkdwn "Latest screenshot"

# 4. Add acknowledgment reaction
slk reactions add --channel "#support" --ts "$MESSAGE_TS" --emoji "white_check_mark"
```

### Event Stream Filtering

```bash
# Only wake downstream workers for message events from one channel
slk events stream --channel "#support" --exclude-self --event-type message

# Keep multiple event classes in one stream
slk events stream --channel "#support" --event-type message,reaction_added

# Also append each matching event to a local NDJSON file
slk events stream --channel "#support" --event-type message -f /tmp/support.events.ndjson
```

### Daemon Event Loop Example

```bash
# Terminal/supervisor 1: cache visible Slack events for 24h by default
SLACK_CLI_ROLE=bot slk daemon run --channel "#_bot-testing"

# Agent loop (queue): claim top-level messages -> process -> ack
while true; do
  event=$(slk events claim --type message --message-kind root --channel "#_bot-testing" --lease 5m)
  cursor=$(echo "$event" | jq -r '.cursor')
  # ... process event ...
  slk events ack "$cursor"
done

# Thread replies can be claimed separately, which is useful for subagent handoff.
slk events claim --type message --message-kind reply --thread "$THREAD_TS" --lease 5m

# Only claim messages that explicitly mention the active Slack user.
slk events claim --type message --mentions-me --lease 5m

# If processing fails, do not ack; the lease expiry makes it claimable again.

# Avoid reacting to the bot's own messages
slk events claim --type message --exclude-self --lease 5m | jq -r '.cursor'
```

## Configuration

### Config File Location

```
~/.config/slack-cli/config.json
```

Or override with `SLACK_CLI_CONFIG` environment variable.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SLACK_CLI_READ_ONLY` | Strict boolean: `true` permits reviewed reads and local operations; unset preserves read/write behavior |
| `SLACK_CLI_ROLE` | Active auth role: `user` or `bot` (default: `user`) |
| `SLACK_USER_TOKEN` | Override user token from config |
| `SLACK_BOT_TOKEN` | Override bot token from config |
| `SLACK_APP_TOKEN` | App-level token for Socket Mode events |
| `SLACK_CLI_CONFIG` | Custom config file path |
| `SLACK_CLI_FORMAT` | Default output format (`json` or `human`) |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error (missing config, invalid tokens) |
| 3 | Authentication error (invalid/expired tokens) |
| 4 | Rate limit exceeded |
| 5 | Network error |
| 6 | Permission denied (missing scopes or `read_only_violation`) |
| 7 | Resource not found (channel, user, message) |
| 124 | Wait timeout, e.g. `events next --timeout` |

## Open

Current online validation and credential limitations are recorded in
[Controlled Slack verification](docs/LIVE_TESTING.md).

- v0.5.0 CLI expansion: P1/P2 implementation, source review, offline
  tests/build/vet, and authorized E2E results from `#_bot-testing` are
  recorded in the [channel acceptance report](docs/e2e/2026-09-16-channel-summary.md).
  Missing scopes and excluded workspace-wide actions remain explicit gaps;
  they are not implied to pass. Public API limitations and the implemented
  command surface are recorded in [CLI coverage](docs/CLI_COVERAGE.md).

- Live follow-up: repeat profile/email-present checks with suitable scopes,
  and investigate repeated directory-lookup timeouts and bot-message search
  visibility. The live record distinguishes these gaps from passing scenarios.

- [Issue #5](https://github.com/kehao95/slack-agent-cli/issues/5): deferred User
  Action Token and contextual search support. The global identity contract,
  read-only policy, ordinary search identity checks, and email availability
  reporting are implemented; see [read-only agent operation](docs/READ_ONLY.md).
  Host context forwarding remains separate work.

## License

MIT
