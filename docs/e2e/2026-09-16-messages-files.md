# Messages and files E2E — 2026-09-16

Target: `#_bot-testing` (`C06MNE12X41`) in team `T027K0ZC9`.

The supplied binary was `/tmp/slk-channel-e2e.cjx1rc/slk`, built for revision
`6a8e2dd`. After the run exposed that exact message reads did not request full
metadata, the source was corrected and a second binary was built at
`/tmp/slk-channel-e2e.cjx1rc/slk-luna`. The patched binary was used for the
getter recheck, explicit clear checks, and subsequent cases. No credentials or
private workspace responses are stored here.

QA’s final gate binary is `/tmp/slk-channel-e2e.cjx1rc/slk-qa-final-20260916`
with SHA-256
`4108dac4c7f3f35dee51c720038c73d38f6aac26a889d3cb50f46fb6df58dba0`.
Its user and bot identity checks and local preview sanity passed. The final
binary also passed a root+reply permalink read: `--thread-ts` returned the
exact reply timestamp and root relationship. Its clear-only metadata edit
returned Slack `no_text` with the actionable content hint and left the root
unchanged; an explicit text plus empty blocks/attachments and metadata clear
succeeded. Both final fixture messages were deleted and verified absent.

The bounded search/download supplement below used the earlier patched
`slk-luna` binary; its resources were separately deleted and verified absent.

| Area | Result | Evidence |
| --- | --- | --- |
| User and bot identity | PASS | Both roles matched the preflight principals and team. |
| Message send and thread | PASS | Bot-created root plus two replies; exact timestamps and complete three-message thread returned. |
| Message permalink get | PASS | Root and reply Slack permalinks with `thread_ts` returned the exact requested timestamp and stable `channel_id`. |
| Rich message edit | PASS | Blocks and legacy attachments were set and read back. |
| Message metadata | PASS | Metadata event type and opaque payload round-tripped after the getter requested `include_all_metadata`. |
| Explicit attachment/metadata clear | PASS | `--attachments '[]'` and `--metadata '{}'` succeeded when sent with explicit text content. |
| Clear-only edit | PASS, rejection path | Final binary surfaced Slack `no_text` with a `--text`/non-empty `--blocks` hint and left the owned message unchanged; content-free clearing itself was not successful. |
| Explicit block clear | INCONCLUSIVE | With explicit text plus `blocks: []`, Slack returned a generated rich-text representation; no custom section block remained. |
| Client message ID | PASS | A bot message sent with a client message ID was created and later deleted. |
| Local message preview | PASS | Inline, file and stdin input, JSON and human output worked without a config; malformed block JSON failed locally. Human output states that rendering is not performed. |
| Channel-scoped search | PASS | User raw message, unified message, and file searches used `in:_bot-testing` plus a unique marker; fresh file indexing succeeded after one bounded retry. Bot search was refused before search. |
| Non-author writes | PASS | User edits/deletes against a bot-owned message were rejected with `cant_update_message`/`cant_delete_message`. |
| File upload and metadata | PASS | UTF-8 and binary fixtures uploaded; info worked by ID and Slack permalink. |
| File read/export/download | PASS | Text JSON, URL raw bytes, binary raw export, download SHA-256 equality, and stdout JSON passed. External file URL input was refused. |
| File destination safety | PASS | Existing export destination was refused; `--force` replaced it; output bytes matched the fixture. |
| File list filters | PASS | Channel plus `--type text` returned the owned text fixture. |
| Read-only reads | PASS | Message thread read and file text read succeeded with read-only enabled. |
| Read-only writes | PASS | Message edit and file delete were refused with `read_only_violation`. |
| Reactions | PASS | User-owned fixture reaction add/list/remove passed. |
| Pins | PASS | User-role pin add/list/remove passed; bot-role calls were skipped after `missing_scope`. |
| Bookmarks | SKIP | Both roles lacked `bookmarks:read`; no bookmark was created. |

Cleanup deleted all 7 messages and all 3 files created by this run, and
ID-specific reads confirmed they were gone. No resources were retained. One
search-file upload had already succeeded at Slack when a local shell `PATH`
variable mistake prevented immediate response parsing; the real returned file
ID was subsequently recorded and deleted, so there is no untracked upload.
The complete
mode-0600 ledger is outside the repository at
`/tmp/slk-channel-e2e.cjx1rc/ledger.jsonl`.
