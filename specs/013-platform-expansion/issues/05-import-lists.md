# Previewed Letterboxd and IMDb list ingestion

Label: needs-triage
State: open
Blocked by: specs/013-platform-expansion/issues/04-series-and-anime.md

Implement LST-001–LST-003 in the [feature specification](../spec.md).

## Acceptance

Publish bounded CSV/import preview/commit APIs and source adapters before implementation. Start with user exports; prove deduplication and no local deletion on list removal. Provider APIs remain conditional on official access.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Verification

Not started. Intended behavior is specified above; no completion evidence is claimed.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.
