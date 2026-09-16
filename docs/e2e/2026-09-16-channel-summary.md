# Channel-scoped P1/P2 acceptance — 2026-09-16

The Human authorized both proposed test rounds, including Canvas and Lists,
using the existing user and bot credentials and only `#_bot-testing`.
Three Luna agents executed offline/preflight, messages/files, and artifacts;
the parent coordinated review and closure.

## Boundary and revision

The implementation starts at `6a8e2dd` on `feat/cli-p1-p2-coverage`. Narrow fixes
found during acceptance are included with this report. The per-area reports
identify test binaries and verification details.

Preflight verified channel `C06MNE12X41` (`_bot-testing`) in team `T027K0ZC9`,
with the existing user and bot already members. **The channel is public.**
Channel-local fixtures therefore have the channel's ordinary visibility; this
run does not claim a private sandbox. No real coworker was invited, and no
other channel, personal setting, usergroup or OAuth grant was modified.
Only recorded test-owned resources were eligible for cleanup. Raw responses
and credentials were excluded from the repository.

## Results

| Area | Outcome | Evidence / limitation |
| --- | --- | --- |
| Build, vet, unit tests | PASS | Offline gate with credential environment scrubbed; smoke expectations updated for the new CLI contract. |
| Identity and target channel | PASS | Both roles authenticated; same team and target-channel membership confirmed without global directory enumeration. |
| Messages | PASS | Send, thread, exact get, permalink, rich edits, metadata round-trip and local preview; see the message report for field-clearing limits. |
| Files | PASS | Text/binary upload, metadata, read/export, byte equality, filtered listing, overwrite protection and deletion. |
| Read-only guard | PASS | Allowed fixture reads and denied writes; zero-network assertions belong to the offline tests. |
| Reactions and pins | PASS | Add/read/remove on owned fixtures; pins required the user role because the bot lacked scope. |
| Channel-scoped search | PASS | User message and file searches found unique fixtures; queries included `in:_bot-testing`. Bot search was refused. |
| Non-author message mutation | PASS, rejection path | The user could not edit or delete the bot-owned fixture. |
| Bookmarks | SKIP | Both tokens lack `bookmarks:read`; no bookmark created. |
| Lists lifecycle | PASS, user role | Create schema, metadata update, row create/read/update, name/key and native column addressing, single/batch row deletion. |
| Lists export | PASS, user role | Completed JSON job; parsed download contained the expected ID, updated marker, select/date values and completeness flag. |
| Lists access changes | PASS, creator/API view | Share/revoke requests and creator metadata verified. |
| Lists cross-role access | INCONCLUSIVE | Bot lacks `lists:read`; public-channel inheritance also limits revoke assertions. |
| Canvas lifecycle | SKIP | Both roles lack `canvases:write`; target channel had no primary Canvas ID for a scoped read-only case. No Canvas created or changed. |
| Bot channel membership cycle | SKIP, scope-limited | Invite of the existing member returned `already_in_channel`; bot leave returned `missing_scope`. Both roles confirmed membership unchanged. No successful leave/reinvite or kick is claimed. |
| Live event reception | SKIP | App token is present, but no isolated Socket Mode consumer/queue is available; no live stream connection or queue claim/ack was performed. |

Permission failures and already-member responses verify error paths, not the
corresponding successful mutation. Offline event tests do not establish live
event delivery.

## Cleanup

All 7 messages, 3 uploaded files, reactions and pins created for the run were removed.
Message reads returned not-found and file metadata reads returned
`file_deleted` for the recorded fixture IDs. Supplementary cases are included
in the message agent's private ledger and per-area report.

The List `F0C25SWL01H` had all test rows and grants removed, then was deleted
with `files.delete` under the creator identity. `files.info` returned
`file_deleted` and the List read returned `list_not_found`. No retained List
container or Canvas remains from the artifacts run.

## Fixes found during acceptance

- Restored compilation of List field normalization and added the missing
  dependency checksum.
- Requested full metadata for exact-message and thread reads.
- Preserved explicit empty arrays and metadata in message updates without
  silently reading and resubmitting message text. Content-free updates rejected
  by Slack now include an actionable hint; regression tests assert omitted
  fields stay omitted.
- Rejected non-HTTP(S) message permalinks and updated stale command smoke
  expectations for partial edits and nested presence commands.

The final source passed the offline gate. Live cases used the immutable
binaries identified in their reports; this is not a claim that every earlier
case was repeated against the final binary. The final binary additionally
passed precise reply-permalink retrieval, safe rejection of a content-free
clear, and explicit-content clearing. Slack's generated rich-text response
means a strictly empty returned blocks array remains inconclusive.

## Evidence

- [Offline checks, identity, and optional channel operations](2026-09-16-offline-preflight.md)
- [Messages, files, reactions, and pins](2026-09-16-messages-files.md)
- [Canvas and Lists execution](2026-09-16-artifacts.md)

Workspace-wide directory searches, personal profile/status/photo/presence/DND
writes, usergroup mutation, and app-grant changes remain outside this run.
Permission-blocked and inconclusive cases are not counted as passing E2E
coverage. The broader [sandbox plan](../CLI_E2E_PLAN.md) remains future work
where its prerequisites are available.
