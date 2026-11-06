# Poll downloads and prepare import handoff

Label: ready-for-agent
State: done
Blocked by: specs/004-acquisition/issues/03-submit-and-reconcile.md

Implement POL-001–POL-009: durable download-task tracking by confirmed client/hash/category/tag identity, fifteen-second scheduled polling, bounded worker use, restart recovery, sanitized transfer/completion/failure history and ownership/configuration changes. Record provisional completed-file evidence and the handoff contract before import implementation. Do not assume a successful submission proves completion or file/movie identity. Preserve original downloads and existing client torrents.

## Verification

Implemented POL-001–POL-009. [Recorded evidence](../verification.md#ticket-04--2026-11-06): full pinned non-root Go tests/vet, native Windows API/storage/pool suites, frontend build, six API lints, tracker consistency, 31 browser journeys plus final affected 7/7, and Linux Compose checks pass. Public API checks use real disposable storage with HTTP simulators; browser tracking responses are simulated. Live transfer/path validation remains optional deployment evidence.

## Comments

Claimed by Codex on 2026-11-06. Polling/evidence contracts published before implementation; import and remote transfer mutations excluded.

Completed on 2026-11-06. The next feature will specify local validation and recoverable imports before implementing any media mutation.
