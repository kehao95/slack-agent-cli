# Isolated CLI P1/P2 E2E plan

Original status: plan only, with testing deferred during implementation.
On 2026-09-16 the Human subsequently authorized execution of both proposed
channel-scoped rounds, including Canvas and Lists, using only the existing
user/bot credentials and `#_bot-testing`. This overrides the dedicated-workspace
prerequisite for that narrower run, not the boundary against unrelated members,
channels, personal settings or OAuth changes. Build/vet/unit checks and the
available live cases are recorded in the
[channel E2E report](e2e/2026-09-16-channel-summary.md).

The full sandbox plan below remains a template for broader coverage; it is not
a claim that every case was executed in the existing workspace.

## Isolation contract

Use a dedicated Slack development workspace with the paid features needed for
Canvas and Lists. Use a separate test app and disposable test identities. A
private channel in a production workspace is sufficient for limited message
and file checks, but **not** for the full suite: profiles, presence, DND,
usergroups, app scopes, and some resources have workspace-wide effects.

Required principals:

- `ACTOR_USER`: disposable user A with a user token for user-only actions.
- `PEER_USER`: disposable user B for membership and resource-access checks.
- `BOT_USER`: the test app's bot, used for bot-supported message/file actions.
- Fixed expected workspace and app IDs. Verify the token principal against
  these IDs before any remote mutation. A mismatch stops the run.

Do not use a coworker, the workspace owner's daily account, a production bot,
an external Slack Connect participant, or production OAuth grants as a test
fixture. Do not create or invite new workspace users as incidental setup.

Create one private, uniquely named channel such as `slk-e2e-<UTC-run-id>`.
Only A, B and the test bot may be members. Exclude real integrations, webhooks,
alerting, and agents. Membership invite/kick tests target B only; no real user
gets a notification. Use existing IDs for every destructive action.

Use synthetic English content, a unique run marker, inert links such as
`https://example.com/`, and no mass mentions. Disable link/media unfurls in
message fixtures. Do not publish public file URLs or share resources outside
the private test channel and test identities.

Run the suite in a disposable container or dedicated OS account with its own
home directory. Set `SLACK_CLI_CONFIG` to that run's config file. The current
metadata cache is derived from the OS home (`.config/slack-cli/cache/TEAM_ID`);
changing the config path or `XDG_CACHE_HOME` alone does not isolate that cache.
Keep tokens in environment variables or mode-0600 files; never print them or
include them in the evidence bundle.
Do not call `auth oauth` during the suite: required scopes are provisioned on
the test app before the run, outside any production app.

## Evidence and recovery ledger

Before the first write, create a mode-0700 run directory and a mode-0600 JSONL
ledger. Record workspace/app/principal IDs, revision, run marker and timestamp.
Record every created resource ID immediately after creation, with its owning
principal, parent channel and cleanup operation. Never discover cleanup targets
by a prefix search and delete all matches.

For each case record:

- command and argument names with credentials removed;
- expected and actual exit code, parsed JSON shape, and relevant field values;
- resource IDs, before/after assertions, and cleanup outcome;
- `PASS`, `FAIL`, `SKIP` (missing scope/feature), or `INCONCLUSIVE` (for example
  search indexing delay). A successful HTTP response alone is insufficient.

If an API fails after it may have created a resource, retain the returned IDs
and inspect only that run's channel/resources before retrying. Do not blindly
repeat a create operation. Bound search polling and request volume; suggested
limits are one worker, at most 3 retries per read, at least 1 second between
polls, and a 30-minute overall budget. Stop on identity drift, unexpected
membership, authorization errors, or cleanup ambiguity. Do not change workspace
settings to make a case pass.

## Phase 0: offline checks, before any Slack call

These are scheduled work, **not checks run during implementation**:

1. Run `go vet ./...`, `go build ./...`, and `go test ./...` in the implementation
   branch. Run formatting checks without changing unrelated files.
2. Traverse the Cobra command tree: each runnable command must have a reviewed
   `commandOperations` entry. Every remote write must be refused in read-only
   mode before config discovery, input-file opening, stdin reads or networking.
3. Use a denying/mock HTTP transport for invalid input and read-only tests. In
   particular, a failed call against a real workspace is not proof of zero
   side effects. Cover aliases and mixed read/write parent commands.
4. Run local message preview without credentials and with networking disabled.
   Exercise inline JSON, `@file`, stdin, malformed JSON, absent block types,
   multiple competing stdin inputs and arbitrary valid Block Kit types.
5. Test resource URL parsing, HTML-to-Markdown conversion, byte limits, file
   no-overwrite behavior and trusted download-host/redirect boundaries using
   fixtures. No URL parser should forward Slack credentials to an arbitrary
   host. Do not fetch remote images during conversion or preview.
6. Validate the full/read-only manifests and OAuth default scopes against
   each other, including token-type restrictions. Do not install manifests
   into a real workspace as an offline check.

## Phase 1: preflight and owned fixtures

1. Verify both active roles with `auth whoami`; confirm expected workspace and
   actor IDs, bot ID and app. Do not silently switch roles after a denied call.
2. Check the test users by canonical ID. Capture only synthetic directory data.
3. Create the private channel as A, record its ID, and verify `is_private` and
   membership. Invite only B and the bot if necessary.
4. Create one root message and two replies using a unique marker; create one
   UTF-8 text file and one binary file with locally recorded byte digests.
5. Create a standalone Canvas, a small List, and one channel bookmark in this
   fixture channel. Record resource IDs before continuing.

The following command sequence is a recipe for the later test operator, not a
script to run against the current workspace. Populate variables only from the
identity checks and ledger above; inspect each response before the next write.

```sh
slk conversations create --name "$RUN_CHANNEL_NAME" --private
# Record the returned channel ID as CHANNEL_ID before continuing.
slk conversations invite --channel "$CHANNEL_ID" --users "$PEER_USER"
slk conversations members --channel "$CHANNEL_ID" --all
slk conversations search --query "$RUN_CHANNEL_NAME" --types private_channel
slk users search --query "$PEER_USER"

slk messages preview --mrkdwn @fixture-message.md --metadata @fixture-metadata.json
slk messages send --channel "$CHANNEL_ID" --mrkdwn - --metadata @fixture-metadata.json --unfurl-links=false --unfurl-media=false < fixture-message.md
# Record the returned timestamp as ROOT_TS.
slk messages get --channel "$CHANNEL_ID" --ts "$ROOT_TS" --include-thread
slk messages edit --channel "$CHANNEL_ID" --ts "$ROOT_TS" --blocks @fixture-blocks.json

slk canvases create --title "$RUN_MARKER" --content @fixture-canvas.json
# Record CANVAS_ID. Keep standalone until direct-grant tests complete.
slk canvases read --canvas "$CANVAS_ID" --format markdown
slk canvases export --canvas "$CANVAS_ID" --format html --output "$RUN_DIR/canvas.html"
slk canvases share --canvas "$CANVAS_ID" --access-level read --users "$PEER_USER"
# Use B's isolated credentials to verify access, then revoke as A.
slk canvases revoke --canvas "$CANVAS_ID" --users "$PEER_USER"

slk lists create --name "$RUN_MARKER" --schema @fixture-schema.json
# Record LIST_ID. Record each item ID after item-add.
slk lists item-add --list "$LIST_ID" --fields @fixture-fields.json
slk lists export-start --list "$LIST_ID" --format json
# Record JOB_ID, poll with a bounded delay, retain the completed download.
slk lists export --list "$LIST_ID" --job "$JOB_ID" --format json --output "$RUN_DIR/list.json"
```

Message fixture files must contain no mentions or external links, and the send
command explicitly disables unfurls.
Add reply fixtures before asserting full-thread results. All personal-setting
commands in the acceptance matrix use A's dedicated credentials and baseline;
none belong in a reusable script that defaults to the operator's normal token.

## Phase 2: CLI acceptance matrix

The command reference in [CLI_COVERAGE.md](CLI_COVERAGE.md) provides the exact
command surface. Build argument files locally rather than interpolating JSON
or content into shell command strings.

| Area | Cases | Required assertions / containment |
| --- | --- | --- |
| Existing channel membership | members; invite B; members; kick B; members; restore B | Only B changes; channel stays private; all IDs match the ledger. Use B as a disposable test account. |
| User discovery | Search A/B by partial synthetic name, exact handle/ID/email; no match; pagination | Stable `user_id`, actual `@username`, separate display name; no fabricated handle; explicit provider and pagination semantics. |
| Channel discovery | Search run channel by name and description; no match; pagination | Only accessible results; the run channel found; canonical channel IDs retained. |
| User pagination | `users list` one page, continuation, `--all` | No duplicated/lost users in a small sandbox; consistent cursor/output shape; bounded retries via mocks rather than forced live throttling. |
| Canvas lifecycle | create; read HTML/Markdown; edit insert/replace/delete; sections lookup; export | Synthetic marker and intended section change survive; original HTML export retained; conversion marked lossy; missing content URL produces a clear error, never preview-as-body. |
| Canvas access | Grant B read/write, inspect as B; revoke, inspect as B | Use standalone Canvas for revocation. Channel-inherited access must not be mistaken for a failed individual revoke. |
| Channel Canvas | Create only on the disposable channel, read and edit synthetic content | No existing channel canvas is replaced; use returned canvas ID. |
| List lifecycle | Create typed schema; update name/description/TODO mode; inspect | Values survive read-back; API restrictions are reported explicitly. Unsupported in-place schema mutation is not counted as passing coverage. |
| List items | Create; update by column ID and display name; read; delete; batch delete | Test text, number, checkbox, date, people, select and multi-select as supported; reject ambiguous/missing columns before writes; preserve untouched fields. |
| List access/export | Share/revoke B; export rows; if download job offered, start/poll/download | Record export job IDs; output matches the small source dataset; inherited access handled explicitly. |
| Bookmarks | list/add/edit/remove | Only the bookmark ID created in this run is changed; existing unrelated bookmarks remain identical. |
| Message get | Read exact message by URL and channel+timestamp; read complete thread; missing timestamp | Returns the requested message (not the nearest historical one); pagination retains root and replies; URL thread context does not replace target identity. |
| Message edit | Change text, then blocks, then metadata | Edit only owned messages; omission follows Slack semantics (text without blocks removes prior blocks); empty arrays clear blocks/attachments and empty object clears metadata; IDs and opaque metadata payload retained. |
| Message send metadata | Send and read back with `--include-metadata` | `event_type`/`event_payload` round-trip; no identity rewriting inside opaque values. |
| Message preview | Inline/file/stdin content, JSON and human output | No auth or Slack request; payload is inspectable; local formatting checks are not advertised as Slack's renderer. |
| File read/export | Read text by ID/URL, download/export binary, output already exists, forced replace | Text matches fixture; binary digest matches; default output is valid JSON; no-overwrite is atomic; no token sent to external links. |
| File input/filter extensions | Any implemented stdin upload, multiple inputs, type/time filters | Each upload recorded individually; exact byte round-trip; filters use synthetic timestamps; partial failures preserve cleanup IDs. |
| Usergroups | Create unique test-only group; members set/add/remove; disable | Sandbox only. No mention of group handle in messages; no default production channels; confirm unchanged members survive add/remove. |
| Profile edit | Change only a synthetic allowed field on A, read back, restore | Disposable A only; preserve every untouched field; no privilege escalation to edit real users. |
| Presence | Set away/auto on A; inspect result | Disposable A only. Presence observation and manual-presence preference differ; do not infer a restorable manual setting from active/away alone. |
| DND | Inspect; snooze A briefly; inspect; end snooze | Disposable A only; do not alter any real user's notification delivery or scheduled DND policy. |
| Photo | Set synthetic image on A; read profile; delete or restore fixture image | Disposable A only. Re-uploading an avatar may not reproduce original crops/assets exactly, so this case is never run on a daily account. |
| Read-only mode | All new reads; each new mutation denied | A permitted read uses only reviewed methods; denial has exit 6; network-zero evidence comes from Phase 0, not production writes. |
| Missing scopes/features | Restricted test token; bot/user mismatch; absent paid feature | Clear actionable error; no alternate credential fallback; mark unavailable cases SKIP, not PASS. |

## Phase 3: restore and remove only owned resources

Run cleanup under the principal that created each resource, in reverse
dependency order. Inspect IDs and expected ownership from the ledger first.

1. Cancel any scheduled message fixtures before their delivery time. This suite
   does not need scheduled messages unless explicitly testing a regression.
2. Restore only profile fields changed by the run. If testing status expiration,
   do not resurrect a previously expired status. On disposable A, end only the
   snooze created by the run and return manual presence to the test baseline.
3. Remove/disable the run's test group; revoke direct Canvas/List grants; remove
   the run bookmark and list rows; delete standalone canvases and uploaded files
   by their recorded IDs. Do not revoke inherited grants on unrelated channels.
4. Delete the run's replies/root messages. Clean list containers only through a
   supported deletion path; if the available API cannot delete the List itself,
   remove it manually in the isolated sandbox and record that step. Do not invent
   a `slackLists.delete` endpoint or assume `files.delete` handles every type.
5. Archive the disposable private channel after checking no unrelated content
   was added. If unexpected content exists, stop automatic cleanup and preserve
   it for review. Do not permanently delete channels as a shortcut.
6. Verify cleanup using the same IDs: deleted resources absent, test channel
   archived, no pending delivery jobs, profile/test-account baseline restored.

If cleanup is incomplete, keep the ledger and report exact retained IDs and
the narrow manual recovery step. Keep private raw responses outside the repo.

## Acceptance report

Publish a sanitized report identifying the revision and test environment,
counts of pass/fail/skip/inconclusive cases, unresolved API behavior, and cleanup
status. Claim full P1/P2 E2E coverage only when every supported case has evidence;
scope-denied or feature-disabled cases remain explicitly unverified.

For a production workspace with no dedicated sandbox, limit the run to explicitly
designated private-channel message/file fixtures and local checks. Skip personal
settings, usergroups, resource sharing, app changes and membership changes that
would notify or affect anyone outside the disposable test principals.
