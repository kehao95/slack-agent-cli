# Homebrew Tap Setup Instructions

This document explains how to set up the Homebrew tap for `slack-agent-cli`.

## Step 1: Create the Homebrew Tap Repository

1. Go to GitHub: https://github.com/new
2. Create a new repository named: `homebrew-tap`
3. Settings:
   - **Owner:** kehao95
   - **Name:** homebrew-tap
   - **Description:** Homebrew tap for slack-agent-cli
   - **Visibility:** Public
   - **Initialize:** Leave unchecked (GoReleaser will populate it)

## Step 2: GitHub Token (No Credentials Needed!)

GoReleaser uses GitHub Actions' built-in `GITHUB_TOKEN`, so you don't need to create any tokens manually. The workflow already has permissions to:
- Create releases
- Push to your homebrew-tap repository

**No manual token creation required!** 🎉

## Step 3: Create Your First Release

Once you've pushed the GoReleaser config and GitHub Actions, create a release:

```bash
# Make sure all changes are committed
git add .
git commit -m "chore: add GoReleaser and CI/CD"
git push origin main

# Create and push a tag
git tag -a v0.1.0 -m "First release"
git push origin v0.1.0
```

GitHub Actions will automatically:
1. Build binaries for all platforms
2. Create a GitHub Release with assets
3. Push the Homebrew formula to `homebrew-tap`

## Step 4: Users Can Install

After the release completes, users can install via:

```bash
# Homebrew (macOS/Linux)
brew install kehao95/tap/slack-agent-cli

# Or with full tap name
brew tap kehao95/tap
brew install slack-agent-cli
```

## Troubleshooting

### If the release fails:

1. Check GitHub Actions logs: https://github.com/kehao95/slack-agent-cli/actions
2. Ensure tests pass: `go test ./...`
3. Verify GoReleaser config: `goreleaser check`

### If Homebrew tap push fails:

The `homebrew-tap` repository must exist and be public before the first release. Create it on GitHub first.

## Testing Locally

Before creating a real release, test GoReleaser locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Run in snapshot mode (no release, just build)
goreleaser release --snapshot --clean

# Check the dist/ folder for built binaries
ls -lh dist/
```

## What Gets Created

After a successful release:

1. **GitHub Release**: https://github.com/kehao95/slack-agent-cli/releases
   - Release notes (auto-generated from commits)
   - Binaries for macOS (Intel + ARM), Linux, Windows
   - Checksums file
   - Source code archives

2. **Homebrew Formula**: https://github.com/kehao95/homebrew-tap/blob/main/Formula/slack-agent-cli.rb
   - Auto-generated Ruby formula
   - References the GitHub Release assets
   - Includes checksums for verification

3. **Users can install** via Homebrew immediately!

## Future Releases

For subsequent releases, just tag and push:

```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

GitHub Actions handles everything automatically! 🚀
