# Controlled Slack verification

Use a designated test channel and explicitly selected identities when validating
the CLI against Slack. Mock tests remain the source of evidence for zero network
requests on denied operations; live tests prove the observed Slack behavior.

The latest P1/P2 acceptance run, including Lists and Canvas permission checks,
is recorded in the [2026-09-16 channel report](e2e/2026-09-16-channel-summary.md).
It distinguishes passing cases from missing scopes and untested operations;
the earlier run below remains historical evidence for its stated revision.

## Protocol

1. Build the revision under test into a temporary binary. Inspect `auth whoami`
   for each role, then use `users info` and `conversations info` to confirm the
   actual bot ID/handle, test channel, and membership. Never print credentials.
2. Keep synthetic content in English and use a unique marker in one test thread.
   For CLI tests, post with the designated test bot. For tests intended to
   trigger a separate Slack agent, post as the authorized human identity because
   that agent may ignore bot-authored messages. Do not accidentally mix these
   two types of test.
3. With read-only disabled, create a small owned message/file fixture. Enable
   `SLACK_CLI_READ_ONLY=true` for reads and blocked-write tests. Test writes only
   against the fixture; do not change channel membership, workspace profiles,
   OAuth grants, or public file sharing as incidental setup.
4. Verify JSON identity fields, accepted/rejected user references, email
   availability, dedicated and raw write rejection, and bot search rejection.
   Use the selected user credential for ordinary search. Check downloaded bytes,
   not merely the download command's exit status.
5. Allow bounded search-index delay. Compare a missing fresh match with the raw
   API and an already indexed channel message. Record an inconclusive fresh
   match separately from a failed API request or CLI data loss.
6. Restore/delete only the test run's own fixtures using the same identity, then
   confirm cleanup. Record the tested revision, passed scenarios, unavailable
   scopes, unresolved observations, and untested paths. Keep private message
   contents, emails, tokens, and raw workspace responses out of the repository.

## Live result: 2026-09-14

Revision: `ef9fe88` (PR #6). Target: `#_bot-testing`. The available test bot was
verified as `@devpfbotservice` (Slack real name `dev_pfbotservice`), with a
separate configured human user identity used for ordinary search.

Passed against real Slack:

- Correct bot/user principal and role; test-channel membership confirmed.
- Canonical ID, complete mention, exact handle, and uppercase handle resolve to
  the same bot. Bare handles, real names, lowercase IDs, and incomplete mentions
  are rejected. A missing canonical user returns exit 7.
- Bot and human `users info` succeed with `email_available:false`, consistent
  with absent emails. Missing email is distinguished from a missing user.
- Bot message creation/edit/delete and reaction add/read/remove work outside
  read-only mode. Normalized history retains the author ID, real handle, and
  exact fixture text; raw history preserves Slack's author ID. Permalinks work.
- Read-only send/edit/delete, reaction/pin changes, raw writes, unknown raw
  methods, the conversation alias, implicit DM opening, uploads, file deletion,
  and OAuth are rejected with exit 6 and `read_only_violation`. Invalid policy
  settings return exit 2. The fixture remained unchanged after refused writes.
- A tiny file uploaded by the bot was inspected and downloaded with read-only
  enabled; downloaded bytes exactly matched the source. User file search found
  the uploaded fixture.
- Bot ordinary search all/messages/files, legacy search, and raw search reject
  with exit 3. User search messages/all, legacy search, and raw search each
  return existing indexed channel messages with correct author IDs/permalinks.
- The test reaction, file, and root message were removed. A subsequent history
  read contained no remaining marker or root message from this run.

Limitations observed:

- Both credentials returned `missing_scope` for `users profile`. The raw API
  confirmed `users.profile:read` is required. Profile-command success remains
  unverified until an appropriately scoped credential is available; this run
  did not change OAuth grants. A live positive email-present case was also
  unavailable with these credentials.
- A repeated full-workspace handle lookup hit its 30-second deadline. The same
  uppercase handle succeeded in 6.1 seconds after a pause. Case-insensitive
  matching worked; repeated enumeration can still time out in this workspace.
  The run did not establish whether throttling or response latency caused it.
  Carry canonical IDs between commands to avoid repeated directory enumeration.
- The fresh bot-authored root message did not appear in user search after
  bounded retries, including the raw API. An author-filtered raw query also
  returned no bot messages, while indexed human messages and the uploaded file
  were searchable. Fresh bot-message search is inconclusive; this run does not
  distinguish indexing delay from Slack-side filtering or visibility behavior.

Socket Mode, full pagination/rate-limit recovery, arbitrary CDN download
redirects, and additional mutation families were not exercised live. Their
existing mock coverage remains separate. User Action Token/contextual search
remains deferred to issue #5.
