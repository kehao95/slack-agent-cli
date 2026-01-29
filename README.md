# slack-cli

**Slack CLI for non-human.**

A command-line interface for AI coding agents to interact with Slack workspaces. Built with Socket Mode support for real-time event streaming.

## Why?

Existing Slack CLI tools lack real-time capabilities. `slack-cli` is the first CLI to support Socket Mode, making it perfect for AI agents, automation scripts, and workflows that need bidirectional Slack communication.

## Features

- **Socket Mode** - Real-time message streaming
- **Bidirectional** - Read and write messages
- **Smart Caching** - Fast channel/user resolution
- **JSON Output** - Machine-readable responses
- **Go Binary** - Single executable, cross-platform

## Quick Start

```bash
# Initialize configuration
slack-cli config init

# Watch for messages in real-time
slack-cli watch --channels "#general" --json

# Send a message
slack-cli messages send --channel "#general" --text "Hello!"

# List recent messages
slack-cli messages list --channel "#general" --limit 10 --json
```

## Installation

```bash
go install github.com/yourusername/slack-cli@latest
```

Or download from [releases](https://github.com/yourusername/slack-cli/releases).

## Configuration

1. Create a Slack App at https://api.slack.com/apps
2. Enable Socket Mode and generate tokens
3. Run `slack-cli config init`

See [docs/DESIGN.md](docs/DESIGN.md) for detailed documentation.

## Use Cases

- AI coding agents (Claude Code, OpenCode)
- DevOps automation
- CI/CD notifications
- Real-time monitoring

## License

MIT - see [LICENSE](LICENSE)
