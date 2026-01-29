# slack-cli

**The first Slack CLI with Socket Mode support - built for AI coding agents and automation.**

A modern, real-time command-line interface for interacting with Slack workspaces. Unlike traditional Slack CLIs, `slack-cli` supports **Socket Mode** for real-time event streaming, making it perfect for AI agents, automation scripts, and interactive workflows.

## Why slack-cli?

Existing Slack CLI tools are either outdated, write-only, or lack real-time capabilities. `slack-cli` fills this gap by combining:

- ✅ **Socket Mode** - First CLI to support real-time event streaming
- ✅ **Bidirectional** - Both read and write operations  
- ✅ **Smart Caching** - Lazy-fetch strategy for instant repeated operations
- ✅ **AI Agent Ready** - JSON output, channel resolution, pipe-friendly
- ✅ **Hybrid Model** - Real-time watch + batch history queries
- ✅ **Go Implementation** - Single binary, fast startup, cross-platform

## Comparison with Other Tools

| Feature | slack-cli | rockymadden/slack-cli | slackcat | slack-term |
|---------|-----------|----------------------|----------|------------|
| Socket Mode (Real-time) | ✅ | ❌ | ❌ | ❌ |
| Read Messages | ✅ | ❌ | ❌ | ✅ (TUI only) |
| Send Messages | ✅ | ✅ | ✅ | ✅ (TUI only) |
| File Upload | ✅ | ✅ | ✅ | ✅ (TUI only) |
| Persistent Cache | ✅ | ❌ | ❌ | ❌ |
| JSON Output | ✅ | ❌ | ❌ | ❌ |
| Agent-Friendly | ✅ | ❌ | ❌ | ❌ |
| Language | Go | Bash | Go | Go |
| Maintained | ✅ | ❌ (2023) | ❌ (2024) | ❌ (2024) |

## Features

### Real-time Event Streaming
Monitor channels, groups, and DMs in real-time using Slack's Socket Mode:

```bash
# Watch all channels
slack-cli watch --channels "all" --json

# Watch specific channels
slack-cli watch --channels "#general,#random" --timeout 60s
```

### Message Operations
Send, edit, delete, and list messages with full formatting support:

```bash
# Send a simple message
slack-cli messages send --channel "#general" --text "Hello world!"

# Send with rich formatting
slack-cli messages send --channel "#general" \
  --text "Deploy succeeded" \
  --color "good" \
  --author "CI/CD Pipeline"

# List recent messages
slack-cli messages list --channel "#general" --limit 20 --json

# Update a message
slack-cli messages update --text "Updated text" --timestamp 1234567890.123456 --channel "#general"
```

### Smart Channel Resolution
Channels are automatically resolved from names to IDs with intelligent caching:

```bash
# First call: fetches channel list and caches
slack-cli messages list --channel "#general" --limit 10

# Subsequent calls: instant lookup from cache
slack-cli messages send --channel "#general" --text "Fast!"
```

### File Operations
Upload and manage files with ease:

```bash
# Upload a file
slack-cli file upload --file report.pdf --channels "#team"

# Upload from stdin
cat logs.txt | slack-cli file upload --channels "#devops" --filetype "text"

# List files
slack-cli file list --json
```

### Reactions and Pins
Manage emoji reactions and pinned messages:

```bash
# Add reaction
slack-cli reactions add --channel "#general" --timestamp 1234567890.123456 --emoji "thumbsup"

# Pin a message
slack-cli pins add --channel "#general" --timestamp 1234567890.123456
```

### Cache Management
Control when and how channel/user metadata is cached:

```bash
# Populate cache incrementally
slack-cli cache populate channels

# Populate all at once
slack-cli cache populate channels --all

# Check cache status
slack-cli cache status --json

# Clear cache
slack-cli cache clear
```

## Quick Start

### Installation

**From source:**
```bash
git clone https://github.com/yourusername/slack-cli.git
cd slack-cli
go build -o slack-cli .
mv slack-cli /usr/local/bin/
```

**From releases (coming soon):**
```bash
# Download latest release
curl -LO https://github.com/yourusername/slack-cli/releases/latest/download/slack-cli-$(uname -s)-amd64
chmod +x slack-cli-$(uname -s)-amd64
mv slack-cli-$(uname -s)-amd64 /usr/local/bin/slack-cli
```

### Configuration

1. Create a Slack App at https://api.slack.com/apps
2. Enable Socket Mode and generate an App Token (`xapp-...`)
3. Add Bot Token Scopes (see [Permissions](#permissions))
4. Install the app to your workspace and copy the Bot Token (`xoxb-...`)

Initialize `slack-cli`:

```bash
slack-cli config init
```

You'll be prompted for your tokens. Configuration is stored in `~/.config/slack-cli/config.json`.

### Permissions

Required Bot Token Scopes:
- `channels:history` - Read messages from public channels
- `channels:read` - List and get info about public channels
- `chat:write` - Send messages
- `groups:history` - Read messages from private channels
- `groups:read` - List and get info about private channels
- `reactions:read` - Read reactions
- `reactions:write` - Add/remove reactions
- `users:read` - Read user info
- `files:read` - Read file info
- `files:write` - Upload files

App Token Scope:
- `connections:write` - Required for Socket Mode

## Use Cases

### AI Coding Agents

Perfect for AI agents like Claude Code and OpenCode:

```bash
# Agent monitors a support channel
slack-cli watch --channels "#support" --json | while read -r event; do
  # Process event and respond
  echo "$event" | jq -r '.text' | your-ai-agent | \
    slack-cli messages send --channel "#support" --text -
done
```

### DevOps Automation

Stream logs to Slack:

```bash
tail -f /var/log/app.log | slack-cli messages send --channel "#alerts" --pre
```

### CI/CD Integration

```bash
# Notify on deployment
slack-cli messages send --channel "#deployments" \
  --text "Deployment to production started" \
  --color "warning"

# Update with results  
if deploy.sh; then
  slack-cli messages send --channel "#deployments" \
    --text "✅ Deployment succeeded" \
    --color "good"
else
  slack-cli messages send --channel "#deployments" \
    --text "❌ Deployment failed" \
    --color "danger"
fi
```

### Interactive Monitoring

```bash
# Monitor multiple channels simultaneously
slack-cli watch --channels "#alerts,#monitoring,#errors" --json | \
  jq 'select(.type == "message") | "\(.channel_name): \(.text)"'
```

## Architecture

`slack-cli` is built with:

- **Go** - Fast, single binary, cross-platform
- **Cobra** - Modern CLI framework
- **slack-go/slack** - Official Slack SDK with Socket Mode support
- **Persistent caching** - Channel/user metadata cached locally with lazy-fetch
- **JSON-first** - All commands support `--json` for machine parsing

See [docs/DESIGN.md](docs/DESIGN.md) for detailed architecture and API reference.

## Development

### Building

```bash
go build -o slack-cli .
```

### Running Tests

```bash
go test ./...
```

### Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Roadmap

- [ ] Binary releases for Linux, macOS, Windows
- [ ] Homebrew formula
- [ ] Docker image
- [ ] Message threading improvements
- [ ] Interactive mode for configuration
- [ ] Shell completions (bash, zsh, fish)
- [ ] Message search support (requires user token)
- [ ] Multi-workspace support

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Built for the new era of AI coding agents and automation. Inspired by existing Slack CLI tools but redesigned from the ground up for real-time, bidirectional communication.

Special thanks to the Slack Go SDK team for their excellent Socket Mode implementation.

---

**Star this repo if you find it useful!** ⭐
