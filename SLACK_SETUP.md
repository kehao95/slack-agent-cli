# Slack Setup Guide for `slk`

`slk` supports OAuth user tokens (`xoxp-`), bot tokens (`xoxb-`), and Slack
client tokens (`xoxc-` plus a cookie). A user token is the recommended default:
it supports workspace search and user-profile updates that bot tokens cannot
perform.

## Choose a manifest

- **Read-only:** [`slack-app-manifest-readonly.yaml`](./slack-app-manifest-readonly.yaml)
  enables conversations, messages, users, user groups, reactions, pins, files,
  Lists, and emoji reads.
- **Full access:** [`slack-app-manifest-full.yaml`](./slack-app-manifest-full.yaml)
  additionally enables conversation management, messages, profile/status
  updates, user-group management, reactions, pins, and file writes.

Some Slack methods have token-type restrictions. In particular, workspace
search and updating your own profile/status require a user token. A bot token
is useful for bot-owned messaging, Socket Mode, and bot-visible resources, but
does not make every command available.

## Create and install the Slack app

1. Open <https://api.slack.com/apps> and choose **Create New App**.
2. Choose **From an app manifest**, select the workspace, and paste one of the
   manifests above into the YAML editor.
3. Create the app, then choose **Install to Workspace** and approve it.
4. Copy the **User OAuth Token** (`xoxp-`) or **Bot User OAuth Token**
   (`xoxb-`) from **OAuth & Permissions**.

Reinstall the app whenever its scopes change; Slack does not add newly declared
permissions to an already-issued token automatically.

## Configure `slk`

Save and verify a user token:

```bash
slk auth login --token xoxp-your-token --verify
slk auth test
```

For an ephemeral configuration, use environment variables instead of writing a
file:

```bash
export SLACK_USER_TOKEN='xoxp-your-token'
slk auth whoami
```

To use a bot token, set the bot role explicitly:

```bash
export SLACK_BOT_TOKEN='xoxb-your-token'
export SLACK_CLI_ROLE=bot
slk auth test
```

The built-in OAuth callback flow can save both user and bot tokens returned by
Slack without printing either credential:

```bash
slk auth oauth \
  --client-id "$SLACK_CLIENT_ID" \
  --client-secret "$SLACK_CLIENT_SECRET"
```

Add the callback URL shown by the command to the app's OAuth redirect URLs. The
authorization request uses a random, single-use state value. Use `--save=false`
only when the returned credentials should be discarded. The defaults request
the full manifest's user and bot scopes; pass `--bot-scopes ''` for a user-only
installation or override both scope flags for a read-only app.

The default config file is `~/.config/slack-cli/config.json`. It and its parent
directory are created with owner-only permissions. You may choose another file
with the global `--config` flag. Environment variables override values loaded
from the file:

- `SLACK_USER_TOKEN`
- `SLACK_BOT_TOKEN`
- `SLACK_APP_TOKEN`
- `SLACK_CLIENT_TOKEN` and `SLACK_CLIENT_COOKIE`
- `SLACK_CLI_ROLE=user|bot`

Never commit credentials. Revoke compromised tokens from the Slack app's
settings immediately.

## Scope reference

The supplied manifests are the source of truth. If configuring an app manually,
the important additions beyond basic conversation history are:

| Capability | User-token scopes | Bot-token scopes |
|---|---|---|
| Conversation reads | `channels:read`, `groups:read`, `im:read`, `mpim:read` and matching `*:history` scopes | Same |
| Conversation management | `channels:write`, `groups:write`, `im:write`, `mpim:write` | `channels:manage`, `groups:write`, `im:write`, `mpim:write` |
| Messages | `chat:write` | `chat:write` |
| User lookup/profile | `users:read`, `users:read.email`, `users.profile:read`, `users.profile:write` | Read scopes only |
| User groups | `usergroups:read`, `usergroups:write` | Same |
| Workspace search | `search:read` | Not available through bot-token search methods |
| Reactions and pins | `reactions:read`, `reactions:write`, `pins:read`, `pins:write` | Same |
| Files | `files:read`, `files:write` | Same |
| Lists and emoji | `lists:read`, `emoji:read` | Same |

## Verify common workflows

```bash
# Read and paginate conversation history.
slk conversations list --all
slk messages list --channel '#general' --all

# Resolve a user, inspect a profile, and search with a user token.
slk users lookup --email alice@example.com
slk users profile --user @alice
slk search all --query 'deployment' --all

# Full-access operations.
slk messages send --channel '#general' --text 'Hello'
slk files upload --file ./report.pdf --channel '#general'
slk usergroups members set --group @oncall --members @alice,@bob

# Call an API that does not have a dedicated abstraction yet.
slk api conversations.info --data '{"channel":"C123"}'
```

## Troubleshooting

- **`missing_scope`:** add the reported scope, reinstall the app, then replace
  the stored token.
- **`not_allowed_token_type`:** switch to a user token for search/profile
  methods, or use a bot-compatible method.
- **Channel or message not visible:** the active identity must be able to see
  that conversation; private conversations normally require membership.
- **`xoxc-` rejected:** a client token also requires `SLACK_CLIENT_COOKIE` (or
  the matching config field).

Run `slk --help` and `slk <resource> --help` for the complete command contract.
See [README.md](./README.md) for examples and [docs/DESIGN.md](./docs/DESIGN.md)
for design details.
