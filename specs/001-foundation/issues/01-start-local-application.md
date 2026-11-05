# 01 — Start a persistent local application

Label: ready-for-agent
State: done
Blocked by: none

## Outcome

A user can start a native Go service, load its embedded React foundation page, and query health and setup status against a real initialized database.

## Context

Read [feature specification](../spec.md), [API contract](../openapi.json), and [ADR-0001](../../../docs/adr/0001-local-modular-application.md). Pin supported stable Go, frontend tooling, modernc SQLite, and Goose versions at implementation time; commit Go sums and frontend lockfile. Use standard Go testing/httptest and the agreed frontend tools.

## Work

- Scaffold Go service, React/TypeScript/Vite frontend, embedded production assets, and a development API proxy. Add repeatable build/typecheck/test commands and dependency manifests.
- Add a pinned OpenAPI 3.1 validator to the contract check and validate the foundation contract before implementing handlers; preserve the documented behavior when correcting schema-only defects.
- Implement bootstrap precedence, private data directory, exclusive process locking, SQLite connection settings, singleton/profile migration tables, migration-history compatibility checking, and startup/shutdown sequencing.
- Implement GET liveness, readiness, setup-state, and pre-setup GET settings. Enforce Host/Origin protection, structured errors, JSON API routing, and mutation middleware for later routes.
- Display an honest unconfigured foundation page with no preselected profile and a setup entry point. Commit build assets only if required by the selected embedding workflow; document the clean-checkout build command.

## Acceptance

Cover FND-001, FND-002, FND-003, FND-004, FND-008 (pre-setup read), FND-011, FND-012 (routing/Host/Origin), and FND-015. Exercise process death/restart to prove lock recovery; use separate processes to prove exclusion. Verify native Windows development and Go build for Linux x64 without CGO. Foundation has no workers or external integration calls.

## Verification

Verified on Windows x64, 2026-11-04:

- `go test ./...`: passed with real temporary SQLite databases and directories, API requests, migration rollback/refusal, all connection pragmas, and child-process locking/recovery tests.
- `go vet ./...`: passed.
- `pnpm run build`: TypeScript checking and embedded production frontend build passed.
- `pnpm run check:api`: pinned Redocly OpenAPI 3.1 validation passed without warnings. POST setup/PUT settings remain explicitly planned operations.
- `pnpm run test:browser`: two Chromium tests passed (unconfigured state/setup entry/reload and structured API 404).
- Native Windows executable built successfully. `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` also passed.
- `python scripts/verify_specs.py`: local specifications and tracker references pass. Runtime databases, caches, dependency directories, and binaries remain ignored.

Toolchain: Go 1.27.3, Node 24.22.1, pnpm 11.25.0. Local portable Go is under `%LOCALAPPDATA%/PiratarrToolchains/go1.27.3/go`; dependencies are pinned in manifests/lockfiles. Runtime data for tests is outside the checkout.

Limitations: Linux runtime/filesystem checks were unavailable because Docker's Linux engine was not running. They remain required for distribution in ticket 04. No real external integrations or full onboarding were exercised; those belong to later tickets. The browser-visible setup route honestly identifies itself as a preview.

## Comments

Prepared from the user-approved development-preparation plan; the repository initially contained specifications only.

2026-11-04: Claimed by Codex on branch `feature`. Implementing the foundation startup slice; setup mutations remain ticket 02.

2026-11-04: Completed startup slice and recorded evidence above. Next unblocked ticket: 02, first-run setup.

2026-11-04: Git workflow updated at the user's request. Startup is published separately on `feature/foundation-01-startup` and integrated into `main` before the setup implementation. Its isolated startup snapshot is revalidated before publication.
