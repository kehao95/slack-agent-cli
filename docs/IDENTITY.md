# User identity contract

This contract applies to dedicated `slk` user-reference arguments and normalized
user output, including messages, search, events, user resources, and authentication.

## Three distinct representations

| Representation | Example | Meaning |
| --- | --- | --- |
| `user_id` | `U012ABC` | Canonical Slack identity; retain it across renames and use it for follow-up calls. |
| `username` | `@alice.zhang` | Exact Slack `User.Name` handle, with an explicit `@` prefix. It is a lookup handle and may change. |
| `display_name` | `Alice Zhang` | Presentation only. It may be missing, duplicated, or changed; never use it as a user reference. |

A display name that happens to look like a handle does not become a username.
Only Slack's actual username field supplies `username`. A message's author alias,
real name, or bot display label must not supply a username.

## Accepted input

- Canonical uppercase user IDs matching `^[UW][A-Z0-9]+$`, such as `U012ABC`.
- Complete Slack mentions containing a canonical ID, such as `<@U012ABC>`.
  This is another encoding of the same ID, not a separate identity type.
- Explicit `@username`, matched case-insensitively against Slack `User.Name` only.
  `@will` is a username. Even `@U012ABC` is explicitly a username, not an ID.

Bare handles (`alice`, `will`), display names, real names, implicit email aliases,
lowercase IDs, and malformed mentions are rejected. Use `@alice` for a handle.
Email lookup remains a separate operation: `slk users lookup --email ...`.

The same parsing rules apply to single users, lists of users, and local event
filters. A batch validates every input before enumerating users and shares one
paginated directory traversal. Direct IDs and mentions never require `users.list`.
Other command setup, such as authentication verification, can still make requests.

User-reference arguments include `--user`, `--users`, usergroup `--members`, and
`--recipient-user`. A DM target written as `--channel @username` uses the same
username rules. Channel and usergroup references have their own contracts.

## Normalized output

```json
{
  "user": "U012ABC",
  "user_id": "U012ABC",
  "username": "@alice.zhang",
  "display_name": "Alice Zhang"
}
```

Scalar `user` fields remain IDs when present; enrichment does not replace them
with names. User resource envelopes may contain a `user` object, whose identity
is in `user_id`. `username` and `display_name` are optional presentation metadata;
missing metadata does not invalidate a known ID. API failures during presentation
lookup must not manufacture a username from a display name or from the ID.

For compatibility, `users list/info/lookup` also retain `id` (the same user ID)
and `name` (Slack's native bare username). New callers should use `user_id` and
`username`; the legacy bare `name` is not accepted directly as a reference.
Profile responses retain the native `profile` object, with `user_id` outside it;
they do not fetch an extra user record solely to discover a handle.
When `--user` is omitted, profile/status operations retain Slack's implicit
authenticated-user behavior and may omit `user_id`; pass an ID to make the
subject explicit.

Nested message identities, including editors and reaction members, also remain
IDs. Original message text and `<@ID>` mentions are preserved in machine output.
Human output may show a display name alongside a real `@username`, but must not
add `@` to a display name and present it as a handle.

Events use the same author fields. Reaction targets use `item_user_id`,
`item_username`, and `item_display_name`, with `item_user` retaining the ID.
New event metadata survives the daemon's local store. Older stored resolved
labels are not promoted to trusted usernames when read back; companion IDs win.

`auth whoami` reports the configured token principal and active role. It does not
identify the person who triggered an agent task. Its username comes from
`auth.test`; no display-name lookup is required for authentication output.

## Native payload boundaries

`slk api` forwards Slack-native request and response objects without resolving
user references. Supply canonical IDs in fields where Slack requires them.
Search query strings, message text, Block Kit JSON, and other opaque Slack
payloads likewise keep Slack's own syntax; the CLI does not rewrite them.

`--raw-json` (or `--resolved-json=false`) preserves native message/search user
fields without identity enrichment. In particular, a native Slack `username`
field may be an author display alias: it is not the normalized `@username`
contract above. Use the raw `user` ID for follow-up calls. Event `--raw` adds the
unaltered source payload alongside the normalized event.

## Migration and agent recipe

This intentionally tightens older behavior:

- Replace bare handles with `@handle`; replace display-name and implicit email
  lookups with an ID, exact handle, or explicit email lookup command.
- Read a display label from `display_name`, not `username`.
- Expect scalar `user` and nested user lists to contain IDs even when resolution
  is enabled. Read `username` separately for a handle.
- Existing consumers retaining `user_id` continue using the same stable ID.

```sh
# Inspect a message and retain its stable author identity.
author_id=$(slk messages list --channel C012ABC --limit 1 | jq -r '.messages[0].user_id // .messages[0].user')
slk users profile --user "$author_id"

# An exact handle is an alternative lookup input.
slk users info --user @alice.zhang
```
