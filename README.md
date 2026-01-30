# slack-agent-cli

> **Slack for non-humans™**

## Why "non-humans"?

Humans have eyes, we have `jq`.
Humans need emojis, we need `thread_ts`.
Humans want a UI, we just want a clean pipe.

**This is not a CLI for you. This is a CLI for your digital entities.**

---

## ⚠️ Human Warning

This tool defaults to JSON output. If your biological eyes find this hard to parse, use the `--human` flag. We recommend upgrading your optic nerve or using `jq`.

## Features

- **Pipe-First Design** - Output is always pure JSON (stdout) while logs go to stderr.
- **Agent-Ready** - Stateless authentication, perfect for LLMs, scripts, and cron jobs.
- **Smart Caching** - Resolves channel names (`#general`) to IDs (`C123...`) locally for speed.

## Quick Start

### For Digital Entities (Default)

```bash
# Get channel history as minified JSON
slack-agent-cli messages list --channel C12345

# Pipe directly into other tools
slack-agent-cli messages list --channel "#general" | jq '.[].text'
```

### For Biological Entities

```bash
# Initialize configuration
slack-agent-cli config init

# List channels in human-readable format
slack-agent-cli channels list --human

# List recent messages
slack-agent-cli messages list --channel "#general" --limit 10 --human
```

## Installation

```bash
go install github.com/kehao95/slack-agent-cli@latest
```

## Configuration

1. Create a Slack App at https://api.slack.com/apps
2. Add User Token Scopes (see [docs/DESIGN.md](docs/DESIGN.md) section 7 for required permissions)
3. Install the app to your workspace and copy the User Token (`xoxp-...`)
4. Run `slack-agent-cli config init`

See [docs/DESIGN.md](docs/DESIGN.md) for the full machine-readable spec.

## Use Cases

### The "Pipeline" Approach

```bash
# Summarize the last hour of #alerts using an LLM
slack-agent-cli messages list --channel "#alerts" --since 1h | llm "Summarize these alerts"

# Auto-reply to specific errors
slack-agent-cli messages search --query "error: deployment" | \
  jq -r '.messages[].ts' | \
  xargs -I {} slack-agent-cli messages reply --channel "#ops" --thread {} --text "Investigating..."
```

## License

MIT
