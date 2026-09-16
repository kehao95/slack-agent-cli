# Offline and channel preflight — 2026-09-16

Revision under test: `feat/cli-p1-p2-coverage` at `6a8e2dd`, with the narrow
compile/checksum fixes and command smoke-contract updates made during this run.

The private test directory is `/tmp/slk-channel-e2e.cjx1rc`; the immutable
post-QA binary is `/tmp/slk-channel-e2e.cjx1rc/slk-qa-final-20260916` (the
earlier binaries remain untouched). The sanitized machine-readable preflight
is `/tmp/slk-channel-e2e.cjx1rc/preflight.json`.
The final binary SHA-256 is
`4108dac4c7f3f35dee51c720038c73d38f6aac26a889d3cb50f46fb6df58dba0`.

The target was resolved from the existing local channel cache, not a global
directory scan: team `T027K0ZC9`, channel `C06MNE12X41` (`_bot-testing`). It is
public, unarchived, and both configured principals are members.

User-role `auth whoami`, `conversations info`, and `conversations members`
succeeded. The user principal is `U06DEM9C9LJ`. Bot-role equivalents succeeded;
the bot principal is `U03750ZF12L`.

`go build .` and `go vet ./...` passed with Slack credentials and team settings
removed from the subprocess environment. After the messages clear-read
adjustment, the final `go test ./...`, `go vet ./...`, and `go build .` passed
with credentials and config overrides absent. The first build exposed missing
`golang.org/x/text` checksums and an undeclared `err` in list field
normalization; the checksum and minimal declaration were added, after which the
fresh binary built successfully. The command smoke test was updated to assert
that message edits accept any supplied editable field while the compatibility
presence parent keeps a runtime user check and the set child remains
independent. Additional regressions cover metadata-only and metadata-clear
wire omission, explicit clear fields, and HTTP(S) permalink parsing.

Final closeout also ran `go fmt ./...` and credential-free `go build ./...`;
both passed without changing tracked files. The immutable binary above was
built from the current messages implementation: explicit metadata/clear fields
are sent without implicit text/history reads, and Slack `no_text` is converted
to an actionable error. Its SHA-256 is
`4108dac4c7f3f35dee51c720038c73d38f6aac26a889d3cb50f46fb6df58dba0`.

The offline test suite made no real Slack request; only the separate read-only
preflight and membership-safety checks called Slack APIs.

The process safety check found no running Slack CLI daemon, Socket Mode
consumer, or other matching Slack process. This local observation does not rule
out a remote consumer. A user-token invite of the already-member bot returned
Slack's `already_in_channel`; a bot-token leave reached Slack but returned
`missing_scope`. The bot remained a member in both user- and bot-token member
listings, so no membership mutation occurred and the leave→reinvite cycle could
not be exercised. No human or other bot was touched.

The app-level token presence check passed without recording its value. Event
reception was still skipped because opening a Socket Mode connection can
consume a remote agent's queue; offline coverage is available and no isolated
event fixture was established. No Socket Mode connection, profile/status/avatar/
DND mutation, usergroup operation, global directory scan, OAuth change, or new
workspace resource was performed.
