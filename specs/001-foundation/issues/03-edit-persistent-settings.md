# 03 — Edit settings without overwriting newer changes

Label: ready-for-agent
State: open
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

Pending implementation.

## Comments

Catalog and active jobs do not exist in this slice. Before introducing them, their feature specifications must constrain root edits to avoid silent media relocation or invalidating active work.
