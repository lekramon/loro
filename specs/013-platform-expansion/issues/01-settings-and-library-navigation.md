# Expandable settings and library navigation

Label: ready-for-agent
State: in-progress
Blocked by: specs/003-search/issues/04-native-1337x.md

Implement NAV-001–NAV-005 in the [feature specification](../spec.md).

## Route contract

Use browser paths without new server endpoints:

- `/settings` opens the default Media Management section.
- `/settings/media-management`, `/settings/profiles`, `/settings/quality`, `/settings/custom-formats`, `/settings/indexers`, `/settings/download-clients`, `/settings/import-lists`, `/settings/subtitles`, `/settings/general` and `/settings/system` select a Settings child.
- `/preview#/home`, `/preview#/movies`, `/preview#/series` and `/preview#/anime` select the corresponding library view. Existing `/preview` hash routes remain valid.

Unknown Settings child paths return to `/settings` with the tree visible; planned library views render an honest planned state and make no API request.

## Acceptance

Route existing controls into the requested settings tree, add honest Series/Anime planned views and preserve header/navigation/setup behavior. Use existing APIs; publish route examples before edits.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Verification

Local checks so far:

- `pnpm run build`: PASS.
- `python scripts/verify_specs.py`: PASS.
- `git diff --check`: PASS.
- Focused browser coverage was added for Settings child routes, inert planned sections, Indexers source tabs and the planned Series view; final browser/Linux/Windows evidence remains pending CI.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.

Claimed by Codex on 2026-11-08 after completing the multi-indexer source tabs. No new backend endpoint or migration is required for this ticket.
