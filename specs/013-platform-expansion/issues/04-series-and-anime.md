# Series, episode catalog and anime classification

Label: ready-for-agent
State: in-progress
Blocked by: specs/013-platform-expansion/issues/03-import-and-operations-gate.md

Implement MED-001–MED-003 in the [feature specification](../spec.md).

## Acceptance

Publish series/episode/numbering APIs, metadata contracts and identity-preserving migrations first. Deliver real catalog and manual episode acquisition before season-pack or monitoring automation.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Verification

Manual episode search slice implemented and verified locally:

- `go test ./internal/sources ./internal/httpapi` — passed on Windows with workspace-local `TMP`/`TEMP` directories.
- Source search test verifies generic text search, season/episode parameters and episode identity preservation.
- Durable episode observation and selection tests verify `observationId`, episode-only identity, idempotent selection storage and movie compatibility.
- Episode acquisition preparation and download submission queueing are verified end to end with episode-only identity and a validated magnet hash.
- Download polling and handoff preserve `episodeId` through completed transfer evidence while movie downloads retain `movieId`.
- Episode-specific import publication remains outstanding for this ticket.

## Migration design

The next additive migration will add `series`, `seasons` and `episodes` tables and a nullable `series_id`/`anime` classification on the existing media-title model without rewriting movie rows or IDs. Series provider identity is unique by provider and provider ID. Seasons belong to one series; episodes belong to one season and use nullable season/episode numbering plus optional absolute numbering. A media file association points to either an existing movie or an episode, never both. The rollback path is migration down before any new rows are created; no movie table or existing movie identifier is dropped or rewritten.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.

Claimed on 2026-11-10. Domain boundary: series, seasons, episodes and specials are additive identities; anime is an explicit classification on a movie or series and never a replacement identity. Movie IDs and APIs remain unchanged.
