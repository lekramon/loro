# Named acquisition profiles, quality definitions and custom formats

Label: ready-for-agent
State: done
Blocked by: specs/013-platform-expansion/issues/01-settings-and-library-navigation.md

Implement POL-001–POL-004 in the [feature specification](../spec.md).

## Acceptance

Before claiming, publish concrete profile/quality/rule APIs and additive migration/rollback contracts. Then implement revision-safe policy editing and candidate explanations; preserve historical snapshots and manual confirmation.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Published contract

Migration 13 is additive and creates `acquisition_profiles`, `quality_definitions`, `custom_formats` and `movie_acquisition_profiles`. The existing singleton settings profile is copied to a named `Default` profile without changing its values; existing acquisition observations retain their immutable snapshots. A stopped-service backup is the rollback contract; no destructive down migration is used by the running product. Older binaries reject the newer migration history.

The public endpoints are:

- `GET/POST /api/v1/acquisition-profiles` and revision-checked `PUT /api/v1/acquisition-profiles/{id}`;
- `GET/POST /api/v1/quality-definitions` for bounded resolution/source and optional size-per-runtime-minute definitions;
- `GET/POST /api/v1/custom-formats` for bounded structured predicates and scores; regex metacharacters and unsupported predicates are rejected;
- `PUT /api/v1/catalog/movies/{id}/acquisition-profile` to assign a profile to a title.

All writes use the existing mutation/header/body protections. Profiles are named acquisition policy, not viewer or subtitle profiles. `allowUpgrades` is persisted as policy metadata only; this ticket does not start automatic upgrades. Missing runtime or release size remains unknown.

## Verification

Completed on branch `feature/013-02-acquisition-policy` at commit `02a8635`.

- `pnpm run build` passed locally.
- `python scripts/verify_specs.py` passed locally (49 API operations).
- GitHub Actions run `42501704775` passed all required jobs: Ubuntu Go tests/vet/build, Windows Go tests/vet/build, browser journeys, and non-root Linux Compose acceptance.
- The storage integration test covers migration of the existing singleton to `Default`, revision-conflict fencing, quality/custom-format persistence and title assignment.
- Docker-based checks were unavailable locally because the Docker Desktop engine was stopped; the required CI Linux container check passed.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.
