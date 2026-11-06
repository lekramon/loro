# Persist selection and establish torrent identity

Label: ready-for-agent
State: done
Blocked by: specs/004-acquisition/issues/01-configure-and-test-download-client.md

Implement ACQ-001–ACQ-009 in the [feature](../spec.md) and [wire contract](../openapi.json). Persist and explain manual selection; support bounded v1 magnet/metainfo identity, review/retry/cancel and restart recovery. No qBittorrent submission, media download/import or background worker is included.

## Verification

Implemented ACQ-001–ACQ-009. See [recorded evidence](../verification.md#ticket-02--2026-11-05): pinned Linux full Go tests/vet, native Windows API/storage/identity tests, frontend build, six OpenAPI checks, specification consistency, 28 browser journeys and final Compose runtime checks pass. Two focused browser journeys pass again against the final executable. API checks use real disposable SQLite/directories and simulated external HTTP services; browser acquisition checks simulate API responses. No live tracker or download submission is claimed.

## Comments

Claimed by Codex on 2026-11-05. Concrete scope published before implementation. The next ticket will specify jobs/submission/reconciliation against these durable intents.

Completed on 2026-11-05. Submission/reconciliation remains [ticket 03](03-submit-and-reconcile.md); no background download or import was added.
