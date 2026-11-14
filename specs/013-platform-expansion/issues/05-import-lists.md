# Previewed Letterboxd and IMDb list ingestion

Label: ready-for-agent
State: done
Blocked by: specs/013-platform-expansion/issues/04-series-and-anime.md

Implement LST-001–LST-003 in the [feature specification](../spec.md).

## Acceptance

Publish bounded CSV/import preview/commit APIs and source adapters before implementation. Start with user exports; prove deduplication and no local deletion on list removal. Provider APIs remain conditional on official access.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket `feature -> develop -> main` publishing workflow.

## Verification

Implemented and verified:

- Public preview/commit/list APIs accept bounded JSON-wrapped `letterboxd-csv` and `imdb-csv` exports with strict mutation guards.
- Preview classifies additions, existing duplicates, ambiguous matches, missing metadata and invalid/duplicate rows before any catalog write.
- Commit registers only unambiguous TMDB matches, is retry-idempotent and persists source/list/item provenance.
- A refresh that omits an existing item leaves its provenance, catalog movie and media ownership untouched; no acquisition or download is started.
- Migration 00018 preserves schema-17 catalog rows and adds immutable preview/provenance tables without rewriting existing identifiers.
- `go test -p 2 ./...`, `go vet -p 2 ./...` and `go build ./cmd/louro` — passed on Windows with workspace-local caches.
- `pnpm run build`, `pnpm run check:api`, `python scripts/verify_specs.py` and `git diff --check` — passed; 52 API operations verified.
- `docker build --tag louro:import-lists .` — passed, including Linux Go tests and vet in the image build.
- `python scripts/verify_container.py --image louro:import-lists --port 18082` — passed Linux non-root Compose, health/setup, persistence and unchanged-media checks.

Provider APIs, scheduled synchronization, credentials, UI and automatic acquisition remain disabled; they require separate contracts and external provider access.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.

Claimed on 2026-11-14. The first implementation slice accepts bounded user-exported CSV content through JSON for `letterboxd-csv` and `imdb-csv`, persists an immutable preview, and commits only unambiguous TMDB matches. Provider APIs, scheduled synchronization, credentials, UI, automatic acquisition and local deletion are explicit non-goals.

## Migration design

Migration 00018 adds import-list provenance, immutable previews and preview rows. Stable source identity is `(source_kind,list_key)` and stable item identity is `(list_id,source_item_key)`. Refreshes upsert observed items but never delete missing rows, catalog titles, media or subtitles. Existing catalog identifiers and rows are not rewritten. Destructive downgrade is unsupported; rollback uses a stopped-service schema-17 backup.
