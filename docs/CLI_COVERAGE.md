# CLI P1/P2 expansion

Implementation branch: `feat/cli-p1-p2-coverage`, based on `ae3900e`.
Status: implementation and authorized channel-scoped acceptance are recorded.
Build, vet and unit tests have now passed; live coverage and scope limitations
are recorded in the [channel E2E report](e2e/2026-09-16-channel-summary.md).
The initial implementation deliberately deferred tests; the Human subsequently
authorized both rounds using the existing user/bot credentials and
`#_bot-testing`, including Canvas and Lists.

## Scope

The CLI priorities agreed on 2026-09-16 are the scope of this change:

- P1: Canvas authoring/read/export/access, List create/update/item mutations,
  user and channel discovery, and channel bookmarks.
- P2: exact message retrieval and rich editing, message metadata, local preview,
  file ID/URL content workflows, profile/presence/photo/DND actions, full user
  pagination and incremental usergroup membership.

Agent sessions, interactive app surfaces, Enterprise administration, Slack
Connect, native Slack Code artifact APIs, OAuth rotation, and a general MCP
client are outside this change. Existing channel lifecycle and membership
commands remain available: `conversations invite` already adds an existing
workspace user to a channel; it does not invite a new account to the workspace.

## API and MCP boundaries

The comparison baseline is Slack's
[official MCP overview](https://docs.slack.dev/ai/slack-mcp-server/).
The CLI uses the Web API and existing user/bot identities; it does not silently
switch credentials or claim to implement the MCP transport.

- User/channel discovery scans visible directory pages and applies explicit
  case-insensitive matching. It is not semantic/RTS search. Names discovered by
  a search remain presentation fields; subsequent writes use canonical IDs.
- Canvas content is obtained through the backing file's authenticated private
  URL. Markdown is a local, lossy HTML conversion, not a lossless Slack-native
  Markdown export. Preserve the HTML export when fidelity matters. Unsupported
  widgets/layout and comments/version history are not synthesized.
- List creation accepts a schema, and row writes can resolve existing columns.
  The public [`slackLists.update`](https://docs.slack.dev/reference/methods/slackLists.update/)
  contract describes name, description and TODO mode; arbitrary in-place column
  schema editing is not documented there. The MCP's broader column-editing
  feature is not claimed as covered by that endpoint.
- Message preview is local payload inspection and structural validation. It
  does not render Slack's UI or create a synchronized native Slack draft.
- File reads expose UTF-8 textual content. Binary files remain downloadable;
  a file reader is not an OCR/PDF/Office extraction engine.
  Buffered file/Canvas reads are limited to 16 MiB. File export to a path or
  explicit raw stdout streams bytes; use those modes for larger binary files.
- Generic Web API calls remain available through `slk api` for uncommon fields,
  subject to token scopes and the exact read-only allowlist.

These boundaries remain explicit in the E2E result: unsupported or unavailable
features are not counted as passing MCP parity.

## Command reference

All examples below describe implemented command paths; they are not a record
of executed commands. The default output remains JSON; use `--human` for
human-readable output.

| Priority | Capability | CLI surface |
| --- | --- | --- |
| P1 | Canvas lifecycle | `canvases list`, `create`, `channel-create`, `read`, `export`, `edit`, `delete`, `sections` |
| P1 | Canvas access | `canvases share --canvas ID --access-level read --users U…`; `canvases revoke` |
| P1 | List authoring | `lists create --name NAME --schema @schema.json`; `lists update --list F…` |
| P1 | List items | `lists item-add --list F… --fields @fields.json`; `item-update --item Rec… --cells @cells.json`; `item-delete`; `items-delete` |
| P1 | List access/export | `lists share`, `revoke`, `export-start`, `export-get`; `export --job ID --output PATH` downloads a completed job |
| P1 | Discovery | `users search --query TEXT`; `conversations search --query TEXT` |
| P1 | Bookmarks | `bookmarks list`, `add`, `edit`, `remove`, scoped by `--channel` |
| P2 | Exact message/thread | `messages get --message URL [--include-thread]`; or `--channel C… --ts TS [--thread-ts ROOT_TS]` |
| P2 | Rich edits and metadata | `messages edit --channel C… --ts TS` with `--text`, `--blocks`, `--attachments`, `--metadata`; `messages send --text TEXT --metadata @metadata.json` |
| P2 | Local preview | `messages preview --mrkdwn @message.md --blocks @blocks.json --metadata @metadata.json` |
| P2 | File content | `files read --file ID_OR_URL`; `files export --file ID_OR_URL --output PATH`; explicit `--raw` for content on stdout |
| P2 | Profile | `users profile set --field title=Engineer`; `--profile @profile.json`; `--custom-field FIELD_ID=value` |
| P2 | Presence/photo | `users presence get --user U…`; `presence set --presence away`; `photo set --file avatar.png`; `photo delete` |
| P2 | DND | `users dnd info`, `team --users U…,U…`, `snooze --minutes 5`, `end --snooze`, `end` |
| P2 | Full user directory | `users list --all --max-retries 3 --page-delay 1s`; continuation through `--cursor` |
| P2 | Incremental group membership | `usergroups members add --group S… --members U…`; `members remove` |
| Existing | Channel membership | `conversations members`, `invite`, `kick` |

Canvas creation accepts `--content @document.json`, where the file contains
`{"type":"markdown","markdown":"# Example"}`. Edits accept the public API's
changes array through `--changes @changes.json`; section lookup uses
`--criteria @criteria.json`. `canvases read/export --format html` retains the
source; the default `markdown` output identifies itself as converted and lossy.
Exports refuse to overwrite an existing file unless `--force` is provided.

List writes accept native typed cell objects with `column_id`, or the shorthand
`{"column":"Column name","value":…}` for supported column types. Names and keys
are resolved from the List schema. Ambiguous columns must be addressed by ID.
Export is an asynchronous job: start it, retain the job ID, then retrieve with
the same format/options. The CLI does not silently poll indefinitely.

Directory search defaults to scanning all visible pages; use `--all=false` and
`--cursor` for controlled page-by-page operation. `--limit` is the API page size,
not a total match cap. Retries honor rate-limit responses. Membership deltas
read the current group then submit a full replacement: they preserve other
members but cannot provide server-side atomic compare-and-swap. Avoid concurrent
writers to a group; removing the final member is rejected (disable the group
instead).

Conversation discovery defaults to public channels; use
`--types private_channel` (or a comma-separated type set) when searching the
isolated private E2E channel.

Message edits omit fields that were not supplied and follow
[`chat.update` semantics](https://docs.slack.dev/reference/methods/chat.update/):
**supplying text without blocks removes the previous blocks**. Supply both when
retaining a block layout while changing fallback text. Explicit `--blocks '[]'`
and `--attachments '[]'` clear those fields; `--metadata '{}'` clears metadata.
Omitted metadata and attachments are retained by Slack. This is not a general
merge-patch API. If Slack rejects a content-free update with `no_text`, the CLI
asks for explicit text or non-empty blocks. It does not read and resubmit the
old text automatically, because that could remove an omitted block layout.

## Authorization

The full manifest and OAuth defaults add `canvases:read`, `canvases:write`,
`lists:write`, `bookmarks:read`, `bookmarks:write`, and `dnd:read` to the matching
user/bot scope sets. Both gain `users:write` for
[`users.setPresence`](https://docs.slack.dev/reference/methods/users.setPresence/);
the user token additionally gains `dnd:write`.
Profile/photo writes use the existing user `users.profile:write` scope.
The read-only manifest gains only the corresponding reads. Editing a manifest
does not upgrade an installed token; the app must be authorized with the
required scopes separately.

Profile changes targeting another user depend on Slack's administrative and
plan restrictions. Presence, photo and DND mutations apply to the authenticated
user. A bot token is not a substitute for a user-only method.

The command policy blocks all new mutations before authentication or input
reads in read-only mode. Canvas/file downloads and List exports are reviewed
reads; local preview does not need credentials. List export-job creation is a
temporary read/export operation, not a mutation of the source List.

## Validation handoff

`bd` is unavailable in this environment. This document and the branch commit
record implementation status; no Beads task is marked tested or closed.
The [E2E plan](CLI_E2E_PLAN.md) describes the original full sandbox suite.
The later channel-only execution has narrower evidence: permission-blocked
Canvas/bookmark operations and workspace-wide personal/group operations are
not covered by a passing List or message test. Consult the per-area reports
before making release claims.

Implementation was split across three Luna agents (artifacts, messages/files,
people/discovery), followed by source cross-review and integration of the
read-only policy, scopes and documentation. Regression test sources were added
for column resolution, partial profile updates, group membership deltas,
message-field clearing, URL parsing and buffered-read limits. These sources
were initially unexecuted and subsequently included in the offline test gate.
Work is retained on the feature branch; main and release tags are unchanged.
