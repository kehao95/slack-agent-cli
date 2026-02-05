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

## Quick Start

### For Digital Entities (Default)

```bash
# Get channel history as minified JSON
slk messages list --channel C12345

# Pipe directly into other tools
slk messages list --channel "#general" | jq '.[].text'
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
- **Simple & stateless:** No OAuth flows, webhooks, or server infrastructure needed
- **Token security:** Keep your token safe - it has the same permissions you do

**User Token** (what this CLI uses):
- Format: `xoxp-...` 
- Represents **you** (the user)
- Perfect for automation, scripts, and AI agents acting on your behalf

**Bot Token** (NOT used by this CLI):
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
slk auth oauth --client-id $SLACK_CLIENT_ID --client-secret $SLACK_CLIENT_SECRET --save
```

This starts a local server on port 8089 with a `/callback` endpoint. Expose it publicly (via your preferred method) and add the callback URL to your Slack app's redirect URIs. With `--save`, the token is automatically saved to config after successful exchange.

## Available Commands

```
slk
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
├── messages        # Message operations
│   ├── list        # Fetch message history
│   ├── send        # Send a message
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
└── emoji           # Emoji operations
    └── list        # List custom emoji
```

## Use Cases

### The "Pipeline" Approach

```bash
# Summarize the last hour of #alerts using an LLM
slk messages list --channel "#alerts" --since 1h | llm "Summarize these alerts"

# Auto-reply to specific errors
slk messages search --query "error: deployment" | \
  jq -r '.matches[].ts' | \
  xargs -I {} slk messages send --channel "#ops" --thread {} --text "Investigating..."
```

### Agent Workflow Example

```bash
# 1. Check recent messages
slk messages list --channel "#support" --since 1h

# 2. Search for specific issues
slk messages search --query "error in:#support"

# 3. Send a response
slk messages send --channel "#support" --thread "$THREAD_TS" --text "Here's the answer..."

# 4. Add acknowledgment reaction
slk reactions add --channel "#support" --ts "$MESSAGE_TS" --emoji "white_check_mark"
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
| `SLACK_USER_TOKEN` | Override user token from config |
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
| 6 | Permission denied (missing scopes) |
| 7 | Resource not found (channel, user, message) |

## License

MIT
