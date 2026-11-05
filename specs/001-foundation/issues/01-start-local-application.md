# 01 — Start a persistent local application

Label: ready-for-agent
State: open
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

Pending implementation. Record commands, environment, results, and any unavailable checks here.

## Comments

Prepared from the user-approved development-preparation plan; no application code exists yet.
