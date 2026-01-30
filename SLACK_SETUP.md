# Slack Setup Guide for slack-agent-cli

This guide walks you through setting up a Slack App with all the necessary permissions for `slack-agent-cli`.

## Quick Setup (Recommended)

### Step 1: Create Slack App from Manifest

1. Go to https://api.slack.com/apps
2. Click **"Create New App"**
3. Select **"From an app manifest"**
4. Choose your workspace
5. Select **YAML** tab
6. Copy and paste the contents of [`slack-app-manifest.yaml`](./slack-app-manifest.yaml) from this repository
7. Click **"Next"** → Review → **"Create"**

### Step 2: Install the App

1. In your new app's settings, go to **"OAuth & Permissions"**
2. Click **"Install to Workspace"**
3. Review the permissions and click **"Allow"**

### Step 3: Copy Your User Token

1. After installation, you'll see **"User OAuth Token"** (starts with `xoxp-`)
2. Copy this token
3. Run `slack-agent-cli config init` and paste the token when prompted

### Step 4: Test the Connection

```bash
slack-agent-cli auth test
```

You should see your user info - you're all set! 🎉

---

## Manual Setup (Alternative)

If you prefer to configure scopes manually:

### 1. Create a Slack App

1. Go to https://api.slack.com/apps
2. Click **"Create New App"** → **"From scratch"**
3. Enter app name: `slack-agent-cli`
4. Choose your workspace
5. Click **"Create App"**

### 2. Add User Token Scopes

Go to **"OAuth & Permissions"** → **"User Token Scopes"** and add:

**Core Scopes (Required):**
- `identify` - Verify user identity
- `channels:read` - List public channels
- `channels:history` - Read public channel messages
- `search:read` - Search messages

**Additional Scopes (Full Functionality):**
- `channels:write` - Join/leave channels
- `groups:read` - List private channels
- `groups:history` - Read private channel messages
- `im:read` - List DMs
- `im:history` - Read DMs
- `mpim:read` - List group DMs
- `mpim:history` - Read group DMs
- `chat:write` - Send messages
- `users:read` - List users
- `reactions:read` - View reactions
- `reactions:write` - Add/remove reactions
- `pins:read` - View pinned messages
- `pins:write` - Pin/unpin messages
- `files:read` - View files
- `files:write` - Upload files
- `emoji:read` - List custom emoji

### 3. Install and Get Token

1. Click **"Install to Workspace"**
2. Review permissions and click **"Allow"**
3. Copy the **"User OAuth Token"** (starts with `xoxp-`)

### 4. Configure CLI

```bash
slack-agent-cli config init
```

Paste your token when prompted.

---

## Troubleshooting

### "Invalid token" error
- Make sure you copied the **User OAuth Token** (not Bot Token)
- Token should start with `xoxp-`

### Missing permissions
- Reinstall the app to your workspace after adding new scopes
- Check that all required scopes are added in **OAuth & Permissions**

### Can't see certain channels/messages
- The CLI can only access channels where the user (you) has access
- Private channels require the user to be a member

---

## Security Notes

- Your token is stored locally in `~/.config/slack-agent-cli/config.json`
- File permissions are set to `0600` (owner read/write only)
- Never commit your token to version control
- You can revoke the token anytime at https://api.slack.com/apps

---

## What the CLI Can Do

Once configured, you can:

```bash
# List channels
slack-agent-cli channels list --human

# Get recent messages
slack-agent-cli messages list --channel "#general" --limit 10

# Search messages
slack-agent-cli messages search --query "error"

# Send a message
slack-agent-cli messages send --channel "#general" --text "Hello!"

# Add a reaction
slack-agent-cli reactions add --channel "#general" --ts "1234567890.123456" --emoji "thumbsup"

# And much more!
slack-agent-cli --help
```

---

## Next Steps

- See [README.md](./README.md) for usage examples
- Check [docs/DESIGN.md](./docs/DESIGN.md) for complete API reference
