# Test metadata, expose live search and persist navigation

Label: ready-for-agent
State: done
Blocked by: specs/011-service-configuration/issues/01-service-configuration.md

Implement CFG-005 through CFG-008 in [specification](../spec.md) and [API contract](../openapi.json).

## Comments

Claimed by Codex on 2026-11-05. Branch feature/02-metadata-test-and-navigation. Read-only check of the running service found configured=true and a successful Inception search (12 results); preview search is mock-only. No token was read or printed.

## Verification

CFG-005 through CFG-008 passed. See [verification evidence](../verification.md): simulator API diagnostics/cache bypass, non-root Linux regression tests/vet, native Windows API tests, 17 browser journeys plus six final affected journeys, production build/contracts and an uncached live query through the existing user service.
