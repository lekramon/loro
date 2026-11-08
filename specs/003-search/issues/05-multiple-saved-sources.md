# Multiple saved release sources

Label: ready-for-agent
State: done
Blocked by: specs/003-search/issues/04-native-1337x.md

Implement SRC-008 through SRC-012 from the [specification](../spec.md) and [API contract](../openapi.json).

## Acceptance

- Preserve the currently configured Torrentio source and its private API key while migrating from one active row to multiple saved rows.
- Store every Prowlarr indexer as a separate Torznab source without making Prowlarr a runtime dependency.
- Keep one explicit active source for existing search, observation, acquisition and preparation behavior; activation must invalidate stale work through a globally increasing revision.
- List and create saved sources through the public API and the single Indexers Settings section, without exposing credentials or creating duplicate endpoint rows on retries.
- Keep inactive-source editing/testing out of scope: activate a source explicitly before using the existing edit/test/search controls.

## Verification

Local implementation checks completed:

- `pnpm run build`: PASS.
- `python scripts/verify_specs.py`: PASS (81 Markdown files, 41 API operations, 35 tickets, 15 foundation criteria).
- `pnpm exec redocly lint specs/003-search/openapi.json --config redocly.yaml`: PASS.
- `git diff --check`: PASS.
- Focused Docker Go tests for `internal/storage` migration preservation/failure and `internal/httpapi` list/create/activate flows: PASS.
- Browser journey could not run locally because native `go` is unavailable and the Docker Engine became unavailable after the focused checks; CI is required for the final browser and Linux non-root evidence.
- Final GitHub Actions run `42497828305` for commit `2bdd405` passed Verify (Ubuntu), Verify (Windows), Browser journeys and Linux container acceptance.

## Comments

Claimed by Codex on 2026-11-07 after the user requested that all Prowlarr sources be created in Lôro before continuing to platform-expansion ticket 01. Branch `feature/003-05-multiple-sources`.

On 2026-11-08 the user clarified the presentation: saved indexers remain separate backend records, but Settings has one Indexers section with a list; indexers are not represented as individual tabs.
