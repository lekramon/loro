# Configure and test qBittorrent

Label: ready-for-agent
State: done
Blocked by: specs/003-search/issues/02-manual-release-search.md

Implement DCL-001–DCL-006 in the [specification](../spec.md) and [API contract](../openapi.json).

## Acceptance

- Persist private credentials with revision checks and explicit replacement when endpoint/username changes.
- Test real cookie-based authentication and supported versions using bounded requests, fresh sessions, classified errors and best-effort logout.
- Preserve previous application state through migration 6 → 7, failures and restarts; client outages do not affect readiness.
- Expose the connection through shared localized Settings with no torrent submission.
- Verify public API/storage on Windows and Linux, production frontend/API contracts, a browser journey and disposable Compose behavior.

## Verification

DCL-001–DCL-006 verified; see [evidence](../verification.md). Frontend/contracts/docs, Linux full Go tests/vet and Compose, Windows API/storage suites, all 26 browser journeys and the final focused client journey passed. Live client credentials were not supplied; simulator checks cover authentication/version behavior without adding torrents.

## Comments

Claimed by Codex on 2026-11-05. Concrete specification/API published before implementation. This ticket prepares client connectivity; selection, durable jobs and submission/reconciliation require later specifications.
