# Configure metadata and align the interface

Label: ready-for-agent
State: done
Blocked by: specs/002-catalog/issues/01-metadata-registration.md

Implement CFG-001 through CFG-004 from [specification](../spec.md) and [contract](../openapi.json).

## Comments

Claimed by Codex on 2026-11-05 at user request. Branch feature/03-service-token-and-jellyfin-layout. User prioritizes this change over scanning.

## Verification

CFG-001 through CFG-004 passed. See [verification evidence](../verification.md): Linux non-root tests/vet, Windows API/application tests, application restart, 15 browser journeys, production build and contracts. Live authentication was not attempted; the interface continues using mock artwork/data.
