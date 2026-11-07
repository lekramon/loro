# Explicit monitoring, upgrades and subtitle automation

Label: needs-triage
State: open
Blocked by: specs/013-platform-expansion/issues/06-native-subtitles.md

Implement Phase 7 in the [feature specification](../spec.md).

## Acceptance

Specify each policy and recovery edge before implementation. Default disabled; prove pause, quota/backoff, idempotency, identity reconciliation and confirmed replacement before enabling effects.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Verification

Not started. Intended behavior is specified above; no completion evidence is claimed.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.
