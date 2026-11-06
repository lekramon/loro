# Submit and reconcile acquisition jobs

Label: ready-for-agent
State: done
Blocked by: specs/004-acquisition/issues/02-select-candidate-and-establish-identity.md

Implement SUB-001–SUB-009: persisted leased jobs, confirmation revisions, category/tags, global torrent-identity exclusion, qBittorrent add and identity reconciliation. A lost submission response must not cause blind resubmission. Polling/completion and import handoff follow as ticket 04.

## Verification

Implemented SUB-001–SUB-009. [Verification evidence](../verification.md#ticket-03--2026-11-06): full pinned non-root Go tests/vet, native Windows API/storage suites, frontend build, six API lints, tracker consistency, 29 browser journeys (plus final focused 3/3), and Linux Compose smoke checks pass. External effects use HTTP simulators; no live transfer/path-mapping claim.

## Comments

Claimed by Codex on 2026-11-06. Concrete submission/recovery contracts published before implementation; polling/import excluded.

Completed on 2026-11-06. [Ticket 04](04-poll-downloads-and-handoff.md) will define completion tracking and import handoff.
