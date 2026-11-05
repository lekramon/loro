# Profile photos and one application shell

Label: ready-for-agent
State: done
Blocked by: specs/011-service-configuration/issues/02-connection-test-and-navigation.md

Implement UI-001 through UI-004 from [specification](../spec.md). Existing [API contract](../openapi.json) and [catalog contract](../../002-catalog/openapi.json) are unchanged. No authentication or server-side profile service added.

## Comments

Claimed by Codex on 2026-11-05. Branch feature/03-profile-photos-and-unified-shell. User requests one initial viewer, photo/pencil editing, library-contained search and consistent minimalist shell.

## Verification

UI-001 through UI-004 passed. See [verification evidence](../verification.md): 20 browser journeys including profile migration/photo persistence/rejection and exact header geometry in both themes, Linux non-root tests/vet, native Windows API tests, build/contracts/spec validation and screenshot review.
