# slack-cli

A command-line interface for AI coding agents to interact with Slack workspaces.

## Overview

`slack-cli` exposes Slack operations as shell commands, enabling AI agents like Claude Code and OpenCode to read messages, send replies, manage reactions, and more.

## Features

- **Socket Mode** for real-time message streaming
- **Hybrid watch model**: `watch` for real-time, `messages list` for batch/history
- **Human-readable** and **JSON** output formats
- **Full Slack API coverage**: messages, reactions, pins, files, users, channels
- **Channel name resolver** that maps `#channel` inputs to IDs

## Quick Start

```bash
# Initialize configuration
slack-cli config init

# Test authentication
slack-cli auth test

# Watch for messages
slack-cli watch --channels "#general" --json

# Send a message
slack-cli messages send --channel "#general" --text "Hello from CLI!"

# List recent messages (auto-resolves #general to its ID)
slack-cli messages list --channel "#general" --limit 10 --json
```

## Documentation

- [Design Document](docs/DESIGN.md) - Full architecture and command reference

## Installation

Coming soon.

## Development

Coming soon.

## License

MIT
