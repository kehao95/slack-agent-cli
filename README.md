# slk

> **Slack for non-humans™**

## Why "non-humans"?

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
# Initialize configuration
slk config init

# List channels in human-readable format
slk channels list --human

# List recent messages
slk messages list --channel "#general" --limit 10 --human
```

## Installation

### Homebrew (macOS/Linux)

```bash
brew install kehao95/slk/slk
```

### Go Install

```bash
go install github.com/kehao95/slk@latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/kehao95/slk/releases)

## Authentication & Configuration

### How It Works

**This CLI uses User Tokens** - It authenticates as **you** and acts on your behalf in Slack. This means:

- ✅ **Acts as you:** Messages, reactions, and actions appear as if you did them
- ✅ **Uses your permissions:** Can only access channels/DMs you have access to
- ✅ **Simple & stateless:** No OAuth flows, webhooks, or server infrastructure needed
- ⚠️ **Token security:** Keep your token safe - it has the same permissions you do

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
   - **Option A:** Run `slk config init` and paste your token
   - **Option B:** Set `export SLACK_USER_TOKEN='xoxp-...'`

**See [SLACK_SETUP.md](./SLACK_SETUP.md) for detailed setup instructions and mode comparison.**

## Use Cases

### The "Pipeline" Approach

```bash
# Summarize the last hour of #alerts using an LLM
slk messages list --channel "#alerts" --since 1h | llm "Summarize these alerts"

# Auto-reply to specific errors
slk messages search --query "error: deployment" | \
  jq -r '.messages[].ts' | \
  xargs -I {} slk messages reply --channel "#ops" --thread {} --text "Investigating..."
```

## License

MIT
