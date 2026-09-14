# Homebrew Formula

GoReleaser generates and updates the slk formula in the separate
[`kehao95/homebrew-slack-agent-cli`](https://github.com/kehao95/homebrew-slack-agent-cli)
tap repository at `Formula/slk.rb`. This directory documents that release flow.

## Installation

```bash
brew install kehao95/slack-agent-cli/slk
```

## How it Works

When a new release tag is pushed, GoReleaser:

1. Builds binaries for all platforms
2. Generates the `Formula/slk.rb` formula for the dedicated tap
3. For a stable release, commits the formula to the tap repository's `main` branch
4. Users install from the tap with `brew install kehao95/slack-agent-cli/slk`

The tap repository, formula path, and prerelease upload behavior are configured
in [`.goreleaser.yaml`](../.goreleaser.yaml).
