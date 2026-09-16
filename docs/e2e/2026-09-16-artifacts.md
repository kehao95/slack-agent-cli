# Artifact E2E execution report — 2026-09-16

This run exercised the artifact commands against only `#_bot-testing`, using
one new disposable Slack List. Canvas creation was attempted under both
authorized roles but did not occur because `canvases:write` was unavailable.
The marker was `Luna artifacts E2E marker 20260916`. Credentials and raw Slack
responses stay outside the repo.

## Preflight

The run used the people agent's sanitized binary and identity/channel preflight
at `/tmp/slk-channel-e2e.cjx1rc`. The binary path, channel ID, actor IDs, and
available scopes are copied only into the mode-0600 local ledger at
`/tmp/slk-artifacts-e2e-20260916/ledger.json`.

The selected actors were verified as members of `#_bot-testing`, and the
available scopes were checked before each resource attempt. Missing scopes were
recorded as `SKIP`; no OAuth grants were requested.

## Fixtures

- Canvas content: `/tmp/slk-artifacts-e2e-20260916/canvas-content.json`
- Canvas edit: `/tmp/slk-artifacts-e2e-20260916/canvas-changes.json`
- Section criteria: `/tmp/slk-artifacts-e2e-20260916/section-criteria.json`
- List schema: `/tmp/slk-artifacts-e2e-20260916/list-schema.json`
- ID/result ledger: `/tmp/slk-artifacts-e2e-20260916/ledger.json` (mode 0600)

## Executed sequence

1. Attempted standalone Canvas creation with the marker under bot and explicit
   user roles. Both calls returned missing `canvases:write`; no Canvas ID was
   created, and no existing or channel-primary Canvas was enumerated or changed.
2. Checked the known channel metadata. Its primary Canvas file ID was empty, so
   Canvas read, section lookup, edit, export, and delete had no resource to run.
3. Created one List with the text, select, and date columns in the fixture and
   recorded its ID and native column IDs immediately. Updated only its
   name/description metadata. Added one marker row, updated that row using
   native column IDs and name-resolved fields, read the known list, deleted the
   row, and exercised the batch-delete path with a second disposable row.
4. Started one List export job and retrieved it after bounded polling. The JSON
   download was 2,622 bytes, parsed successfully, and contained the known List
   ID, updated marker, `done` status, and `2026-09-16` date; no raw private
   response was recorded.
5. Exercised share/revoke only for the disposable resource, the existing test
   channel, and the current bot actor. Creator-role metadata showed the grants
   before and after cleanup; cross-role reads remained `INCONCLUSIVE` because
   the bot lacked `lists:read` and channel access can be inherited.
6. Slack has no public `slackLists.delete` method. After row and access cleanup,
   used the documented `files.delete` route for the owned List file and verified
   the known ID with `files.info` and a List read.

## Results

Status values are `PASS`, `FAIL`, `SKIP`, or `INCONCLUSIVE`. IDs below are
sanitized resource identifiers; the complete local ledger is mode 0600.

- Revision: preflight binary supplied for the current feature branch
- Canvas: `SKIP` — `canvases:write` was missing for both bot and user roles, so
  no Canvas was created or modified.
- List: `PASS` — created `F0C25SWL01H` with text/select/date columns; native
  columns were `Col0C2FPJUS7N`, `Col0C27A0T953`, and `Col0C2E22U4RX`.
  Metadata update, typed name/key item creation, native/name-resolved update,
  known-list/item reads, and both item-delete paths passed. Share/revoke API
  calls passed and creator-role metadata showed the expected grants before and
  after cleanup. Cross-role bot reads were not proved because that credential
  lacked `lists:read`; public-channel inheritance also makes a strict revoke
  access assertion `INCONCLUSIVE`.
- Export job: `PASS` — `Le0C2C1RFM7C` completed as JSON and downloaded to the
  mode-0600 temporary output; parsed bytes contained the known List ID, updated
  marker, `done` status, and `2026-09-16` date.
- Bookmark operations: `SKIP` — `bookmarks:read` was missing for the user
  credential, so no bookmark mutation was attempted.
- Cleanup: `PASS` — rows and grants were removed, then the documented
  `slk files delete --file F0C25SWL01H` command deleted the owned List file.
  `files.info` for the known ID returned `file_deleted`, and a subsequent
  `lists items` check returned `list_not_found`.
- Channel Canvas check: `PASS` — `conversations.info` showed an empty primary
  Canvas file ID and only the existing files tab, so no channel Canvas read or
  export was attempted.
- Scope limitation: the credentials reported `canvases:write` and
  `bookmarks:read` as unavailable. No OAuth grant or unrelated workspace
  resource was changed.
