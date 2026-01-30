# Slack Setup Guide for slk

This guide walks you through setting up a Slack App with the necessary permissions for `slk`.

## Choose Your Mode

**Read-Only Mode** - For viewing and searching only (recommended for most users)
- ✅ List channels, messages, users
- ✅ Search messages
- ✅ View reactions, pins, files
- ❌ Can't send messages or modify anything

**Full Access Mode** - For automation and writing
- ✅ Everything from read-only mode
- ✅ Send, edit, delete messages
- ✅ Add/remove reactions
- ✅ Pin/unpin messages
- ✅ Upload files
- ✅ Join/leave channels

---

## Quick Setup (Recommended)

### Step 1: Create Slack App from Manifest

1. Go to https://api.slack.com/apps
2. Click **"Create New App"**
3. Select **"From an app manifest"**
4. Choose your workspace
5. Select **YAML** tab
6. Copy and paste the appropriate manifest:
   - **Read-Only:** [`slack-app-manifest-readonly.yaml`](./slack-app-manifest-readonly.yaml)
   - **Full Access:** [`slack-app-manifest-full.yaml`](./slack-app-manifest-full.yaml)
7. Click **"Next"** → Review → **"Create"**

### Step 2: Install the App

1. In your new app's settings, go to **"OAuth & Permissions"**
2. Click **"Install to Workspace"**
3. Review the permissions and click **"Allow"**

### Step 3: Copy Your User Token

1. After installation, you'll see **"User OAuth Token"** (starts with `xoxp-`)
2. Copy this token
3. Run `slk config init` and paste the token when prompted

### Step 4: Test the Connection

```bash
slk auth test
```

You should see your user info - you're all set! 🎉

---

## Manual Setup (Alternative)

If you prefer to configure scopes manually:

### 1. Create a Slack App

1. Go to https://api.slack.com/apps
2. Click **"Create New App"** → **"From scratch"**
3. Enter app name: `slk`
4. Choose your workspace
5. Click **"Create App"**

### 2. Add User Token Scopes

#### For Read-Only Mode

Go to **"OAuth & Permissions"** → **"User Token Scopes"** and add:

- `identify` - Verify user identity
- `channels:read` - List public channels
- `channels:history` - Read public channel messages
- `groups:read` - List private channels
- `groups:history` - Read private channel messages
- `im:read` - List DMs
- `im:history` - Read DMs
- `mpim:read` - List group DMs
- `mpim:history` - Read group DMs
- `search:read` - Search messages
- `users:read` - List users
- `reactions:read` - View reactions
- `pins:read` - View pinned messages
- `files:read` - View files
- `emoji:read` - List custom emoji

#### For Full Access Mode (adds to read-only)

Add these additional scopes:

- `channels:write` - Join/leave channels
- `chat:write` - Send/edit/delete messages
- `reactions:write` - Add/remove reactions
- `pins:write` - Pin/unpin messages
- `files:write` - Upload files

### 3. Install and Get Token

1. Click **"Install to Workspace"**
2. Review permissions and click **"Allow"**
3. Copy the **"User OAuth Token"** (starts with `xoxp-`)

### 4. Configure CLI

```bash
slk config init
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

- Your token is stored locally in `~/.config/slk/config.json`
- File permissions are set to `0600` (owner read/write only)
- Never commit your token to version control
- You can revoke the token anytime at https://api.slack.com/apps

---

## What the CLI Can Do

Once configured, you can:

```bash
# List channels
slk channels list --human

# Get recent messages
slk messages list --channel "#general" --limit 10

# Search messages
slk messages search --query "error"

# Send a message
slk messages send --channel "#general" --text "Hello!"

# Add a reaction
slk reactions add --channel "#general" --ts "1234567890.123456" --emoji "thumbsup"

# And much more!
slk --help
```

---

## Next Steps

- See [README.md](./README.md) for usage examples
- Check [docs/DESIGN.md](./docs/DESIGN.md) for complete API reference
