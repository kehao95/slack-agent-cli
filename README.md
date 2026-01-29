# slack-cli

**Slack CLI for non-human.**

A command-line interface for AI coding agents to interact with Slack workspaces. Built with Socket Mode support for real-time event streaming.

## Why?

Existing Slack CLI tools lack real-time capabilities. `slack-cli` is the first CLI to support Socket Mode, making it perfect for AI agents, automation scripts, and workflows that need bidirectional Slack communication.

## Features

- **Socket Mode** - Real-time message streaming (planned)
- **Bidirectional** - Read and write messages
- **Smart Caching** - Fast channel/user resolution
- **JSON Output** - Machine-readable responses
- **Go Binary** - Single executable, cross-platform

## Quick Start

```bash
# Initialize configuration
slack-cli config init

# List channels
slack-cli channels list --json

# List recent messages  
slack-cli messages list --channel "#general" --limit 10 --json

# Watch for messages in real-time (coming soon)
slack-cli watch --channels "#general" --json
```

## Installation

```bash
go install github.com/yourusername/slack-cli@latest
```

Or build from source:
```bash
git clone https://github.com/yourusername/slack-cli
cd slack-cli
go build ./...
```

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
