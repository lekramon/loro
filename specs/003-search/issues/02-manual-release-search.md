# Search and explain movie release candidates

Label: ready-for-agent
State: done
Blocked by: specs/003-search/issues/01-configure-and-test-source.md

Implement REL-001–REL-006 in the [specification](../spec.md) and [API contract](../openapi.json). Use the existing source transport and catalog/settings seams. Torrent retrieval, persisted selection and client submission remain later acquisition work.

## Acceptance

- Search a registered movie using declared capabilities, bounded paging/category and source revision isolation.
- Normalize conservative unknowns and explain all profile dimensions without filtering candidates.
- Reach release search through registration and the offline registered catalog in the shared Library UI.
- Verify through public API, real disposable storage, source/TMDB simulators, Windows and Linux checks, and a browser journey; preserve source configuration regression coverage.

## Verification

REL-001–REL-006 verified. See [ticket 02 evidence](../verification.md): production frontend/API lint/spec checks, Linux non-root full Go tests/vet and Compose, native Windows API/source suites, 25 browser journeys and focused final release-search journeys all passed. Live Torznab validation remains unavailable without an endpoint/key; simulators cover the adapter. No submission or media changes occur in this ticket.

## Comments

Claimed by Codex on 2026-11-05. Concrete REL contracts published before implementation; no further user answers required.

Completed by Codex on 2026-11-05. No schema change; acquisition selection and qBittorrent remain the next feature.
