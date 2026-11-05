# Specify existing-file inventory and association boundaries

Label: ready-for-agent
State: done
Blocked by: specs/002-catalog/issues/01-metadata-registration.md

## Outcome

Publish manual scan scheduling/restart, bounded rooted traversal, inventory identity, review boundary and additive migration contracts before implementation.

## Verification

Concrete SCN-001 through SCN-007 contracts are recorded in [specification](../spec.md) and [wire contract](../library-openapi.json). Association is explicitly deferred to ticket 04 with no automatic identity claims. OpenAPI and repository link/dependency checks pass. These are intended behaviors, not implementation evidence.

## Comments

Completed by Codex on 2026-11-05 after the user requested the next phase. Implementation follows on its own branch in ticket 03.
