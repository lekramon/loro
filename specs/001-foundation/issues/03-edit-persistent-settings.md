# 03 — Edit settings without overwriting newer changes

Label: ready-for-agent
State: done
Blocked by: specs/001-foundation/issues/02-complete-first-run-setup.md

## Outcome

A configured user can edit the profile and local roots, reload to see persisted changes, and receive a clear conflict when another tab has already changed the settings.

## Work

- Implement PUT settings exactly as the [API contract](../openapi.json) defines. Require the current revision and increment it inside the same transaction as all rows.
- Reuse the setup validators. Invalid input, stale revisions, and storage faults leave all previous values intact.
- Add the settings screen with ranking, field errors, saved revision handling, and explicit reload on 409. Keep bootstrap paths, listener, and worker count out of this mutable resource.
- Add the browser setup/edit/reload journey and a controlled concurrent-tab conflict scenario.

## Acceptance

Cover FND-006, FND-007, FND-009, FND-010, FND-012, and FND-013. Two simultaneous updates with the same revision yield one success and one conflict. A storage failure cannot increment revision or partially replace profile rows. Use real disposable SQLite, not an in-memory fake repository.

## Verification

Verified on Windows x64, 2026-11-04:

- `go test ./...`: passed after implementation. Targeted API/storage regression tests also passed after adding a deterministic concurrency boundary: both requests read the same revision before either write, yet only one commits.
- Public API tests verify revision increment, stale 409 responses, atomic directory/profile replacement, sorted resolutions, preserved audio ranking, restart persistence, setup-required behavior, and unchanged user files.
- Invalid revisions, profiles, roots, JSON, bootstrap-only fields, and oversized requests preserve all saved values and revision. Setup still rejects the update-only revision field.
- A real SQLite trigger failing the final audio insert rolls back revision, timestamp, directories, deleted profile rows, and replacements. Read-only writes return 503; clearing the fault allows a retry with exactly one revision increment. Unexpected errors do not expose SQL details.
- `pnpm run build`, `go vet ./...`, and `pnpm run check:api`: passed. Windows executable and Linux x64 CGO-free cross-build passed.
- `pnpm run test:browser`: three Chromium tests passed, covering onboarding, prefilled editing, reranking, field feedback, save/reload, a conflicting second tab, explicit reload, cancellation, and unchanged existing media.

Linux runtime checks remain pending for ticket 04: the Docker Linux engine pipe is unavailable. No metadata credentials or real external integrations were accessed in this implementation.

## Comments

Catalog and active jobs do not exist in this slice. Before introducing them, their feature specifications must constrain root edits to avoid silent media relocation or invalidating active work.

2026-11-04: Claimed by Codex on `feature/foundation-03-settings`. Implementing atomic revision-checked updates and settings editing using the existing setup validators.

2026-11-04: Implementation and required native/API/browser checks complete. Publishing this ticket branch and integrating into `main` under the agreed workflow. Ticket 04 is the next unblocked foundation task.
