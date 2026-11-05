# 02 — Complete first-run setup through the browser

Label: ready-for-agent
State: done
Blocked by: specs/001-foundation/issues/01-start-local-application.md

## Outcome

A user explicitly chooses local directories, accepted/preferred resolutions, and ranked audio choices, then saves a single persistent profile through the web wizard.

## Context

Read [feature specification](../spec.md), [API contract](../openapi.json), and the root glossary. Use the foundation's settings tables and transaction boundary rather than introducing a second configuration store.

## Work

- Bundle ISO 639-1 codes with English names and expose them through setup state; include original audio as a distinct profile token.
- Implement path canonicalization and disposable write probes, rejecting overlapping roots and data-directory conflicts.
- Implement POST setup and complete GET settings, schema/service validation, atomic initial revision 1, and setup race protection. Two concurrent setup requests result in one success and one 409.
- Build the wizard with editable text paths, resolution set, preferred resolution, audio ordering, review, field feedback, and navigation to saved settings.
- Cover strict JSON decoding, duplicate keys, unknown fields, body bounds, content type, and required mutation header.

## Acceptance

Cover FND-005, FND-006, FND-007, FND-008 (setup repetition), FND-010 (setup transaction failure), FND-012 (mutation input), FND-013 (onboarding), and FND-015. Check symlink/reparse-point resolution where supported. Assert probe cleanup and preservation of user files. Restart with the same real temporary database and verify profile order and canonical paths.

## Verification

Verified on Windows x64, 2026-11-04:

- `go test ./...`: passed. Public API integration tests use real SQLite and temporary folders to cover explicit profile requirements, canonical roots, saved revision 1, audio ranking, resolution sorting, restart persistence, and setup repetition.
- Two simultaneous POST requests produce exactly one 201 and one 409, with the winning profile saved intact.
- Invalid/duplicate profile values, missing/relative/file/nested/data-overlapping roots, malformed/duplicate/unknown/trailing JSON, and known-length/chunked oversized bodies are rejected without saving settings.
- Real Windows read-only directory ACL test passed. Directory symlink resolution and alias-overlap rejection passed; probes are removed and existing files preserved.
- Real SQLite trigger failure during the final profile-table write returns 500 and rolls back every table. Read-only database writes return 503. Removing the fault allows a successful retry.
- `go vet ./...`, `pnpm run build`, and pinned Redocly `pnpm run check:api`: passed.
- `pnpm run test:browser`: three Chromium tests passed, including required choices with no defaults, audio reranking, invalid-path feedback, saved settings/reload, and a second tab's setup conflict/reload.
- Windows executable and Linux x64 CGO-free cross-build passed. Runtime data remains outside the checkout.

Linux runtime and mode-bit permission tests remain pending because Docker's Linux daemon was unavailable. Editing saved settings remains ticket 03; the saved-settings screen is read-only in this slice. No real integrations are configured or invoked.

## Comments

No integration credentials are requested in foundation setup. No quality/audio values are silently selected for the user.

2026-11-04: Claimed by Codex on `feature` after ticket 01. Existing uncommitted startup changes are preserved.

2026-11-04: Completed setup slice; ticket 03 (revision-safe settings editing) is now unblocked.

2026-11-04: Setup is published separately on `feature/foundation-02-setup`, based on the integrated startup implementation, then integrated into `main`. Subsequent tickets follow the same one-feature-branch-per-implementation workflow.
