# Read-only agent operation

Set `SLACK_CLI_READ_ONLY=true` to prevent remote Slack mutations through `slk`,
including when the selected credential has write scopes. The setting is absent
by default, preserving existing read/write behavior. There is no CLI flag or
config field that overrides an enabled environment policy.

Accepted values follow Go's strict boolean parser: `1`, `t`, `T`, `true`, `TRUE`,
`True` enable it; `0`, `f`, `F`, `false`, `FALSE`, `False` disable it. A present
empty value, whitespace, or another value is a configuration error (exit 2).

## Boundary

Read-only refers to remote Slack state. Local token saves, cache population and
cleanup, file downloads, SQLite event writes, queue claims/acks, and daemon state
remain available. Read commands still need valid credentials and Slack scopes.
`auth login --verify` may call `auth.test`; `auth oauth` is blocked because it
exchanges a code for credentials and can change an installation's grants.

Commands and their aliases are checked before auth discovery, cache refresh,
input-file/stdin reads, or OAuth listener startup. A second shared guard covers
SDK, generic `api`, bespoke Lists, cookie-authenticated, and Socket Mode HTTP
requests. Unknown methods and future unreviewed commands fail closed. HTTP GET
and a method's name suffix are not evidence that it is read-only.

Blocked operations produce `read_only_violation: blocked operation <operation>`
on stderr and exit 6. Diagnostics name the operation without request payloads,
tokens, cookies, or private download URLs. For example:

```text
$ SLACK_CLI_READ_ONLY=true slk api chat.postMessage --data @request.json
Error: read_only_violation: blocked operation chat.postMessage
```

The reviewed method allowlist is maintained in
[`internal/policy/policy.go`](../internal/policy/policy.go). It includes identity,
conversation/history, user/profile/presence, user group membership, emoji,
pin/reaction, file metadata, permalink/scheduled-message reads, ordinary search,
List item reads/exports, Canvas section lookup, bookmarks and DND inspection.
Directory-backed user/channel search uses existing directory read methods.
Local message preview does not initialize credentials or a Slack client.
All other methods, including upload stages, profile/status
updates, reactions, membership changes, and read-marker changes, are denied.

Two transport exceptions are deliberately narrow:

- `apps.connections.open` obtains a Socket Mode connection URL. Event delivery
  and empty acknowledgments remain allowed; the CLI does not send interactive
  response payloads through acknowledgments.
- `files download` grants a private GET capability for HTTPS
  `files.slack.com/files-pri/` URLs. Each redirect must remain within that origin
  and path prefix. Unreviewed CDN hosts, API URLs, or other paths fail with
  `files.download`; downloads with such redirects need a future allowlist review.

List export's `slackLists.download.start` and `slackLists.download.get` are
reviewed read methods requiring `lists:read`. Starting an export produces a
temporary download job; it does not edit the source List, its rows or grants.
Canvas/file exports to local paths are local writes and remain available.

See Slack's [Socket Mode method](https://docs.slack.dev/reference/methods/apps.connections.open/)
and [private file URL documentation](https://docs.slack.dev/reference/objects/file-object/).

`--channel @username` normally calls `conversations.open`. In read-only mode it
is rejected even on a read command. Use an existing `D...` conversation ID for
DM history, or find it with `conversations list --types im` / `users conversations`.
User lookup with `--user @username` remains a read operation.

This is a CLI guardrail, not token downscoping or a shell sandbox. The
[read-only manifest](../slack-app-manifest-readonly.yaml) limits permissions at
Slack's boundary. A host that must prevent arbitrary shell code from using or
changing credentials needs to hold those credentials outside the agent's shell.

## Identity, email, and ordinary search recipe

Use the [global identity contract](IDENTITY.md): canonical uppercase `U...` /
`W...` IDs, strict `<@ID>` mentions, or explicit `@username` inputs. Display
names are presentation only. Normalized output preserves IDs, separates
`username` (`@handle`) and `display_name`, and leaves raw Slack payloads intact.

```bash
export SLACK_CLI_READ_ONLY=true
export SLACK_CLI_ROLE=user

# Read history and retain an actual author's stable ID for follow-up calls.
history=$(slk messages list --channel C12345 --limit 10)
user_id=$(printf '%s' "$history" | jq -er 'first(.messages[] | (.user_id // .user) | select(type == "string") | select(test("^[UW][A-Z0-9]+$")))')

# Email can be absent even when the user/profile was found successfully.
slk users info --user "$user_id" | jq '.user | {user_id, username, display_name, email_available, email}'
slk users profile --user "$user_id" | jq '{user_id, email_available, profile}'

# Ordinary search uses the selected user's visibility and search:read scope.
slk search messages --query "from:<@$user_id> deployment" --all
slk search files --query 'quarterly report'

# Alternative when a known exact Slack handle is the starting point:
slk users info --user @alice
```

`users info`, `users list`, and `users lookup` expose `email_available` on each
normalized user; `users profile` exposes it alongside `profile`. False means
Slack returned no usable email, while the lookup itself succeeded. A missing
user or a failed API request remains an error. Human output explains that
`users:read.email` is required for email access but that Slack may omit email
for other reasons. No email is inferred from a name or lookup input. See the
[Slack user object contract](https://docs.slack.dev/reference/objects/user-object/).

`search all/messages/files`, legacy `messages search`, and raw
`api search.all/search.messages/search.files` require the active user identity.
A selected bot role (or an `xoxb` credential in the user slot) fails before any
network call with an actionable authentication error (exit 3). Storing both
tokens does not enable fallback. Legacy client tokens retain their configured
cookie through the shared client. Slack permission/rate-limit errors retain
their normal classifications. See [ordinary search requirements](https://docs.slack.dev/reference/methods/search.messages/).

User Action Token and contextual search are deferred to
[issue #5](https://github.com/kehao95/slack-agent-cli/issues/5). There is no new
action-token option or contextual command; `assistant.search.context` is not in
the read-only allowlist. Host event-context forwarding is separate work.

`users search` and `conversations search` are directory-backed discovery, not
contextual/semantic search. They do not require or consume a User Action Token.

## Message metadata recipe

`messages list --include-metadata` remains within the existing read-only
history and thread-reply boundary. It asks Slack to include full metadata on
each returned message and does not require a new scope or credential. Metadata
is optional: use it only when a returned message actually has an execution or
other metadata payload.

```bash
SLACK_CLI_READ_ONLY=true slk messages list \
  --channel C12345 --include-metadata --raw-json --all \
  | jq '.messages[] | select(.metadata? != null) | .metadata'
```

Message `metadata` (`event_type` and opaque `event_payload`) is different from
the response's `response_metadata`, which holds cursor pagination state.
`--raw-json` does not itself ask Slack for full message metadata. The option
does not create an ID, trace, or export, and missing metadata is a valid result.

## Verification

`cmd/policy_test.go` runs the actual Cobra command tree in isolated subprocesses
with fake credentials and intercepted HTTP, checking exit codes and zero
requests for denied commands. Client tests cover SDK/raw/Lists, retries,
pagination, cookies, upload stages, Socket Mode setup, and download redirects.
Search tests cover selected identity and legacy output; user tests distinguish
missing email from failed lookup. No live Slack mutation is needed for this suite.
