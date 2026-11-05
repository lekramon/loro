# Configure and test one Torznab source

Label: ready-for-agent
State: done
Blocked by: specs/002-catalog/issues/04-associate-and-review-files.md

Implement SRC-001 through SRC-007 from [specification](../spec.md) and [API contract](../openapi.json).

## Comments

Claimed by Codex on 2026-11-05. Branch feature/01-torznab-source-configuration. Lean-build and migration skills; release search is the next ticket.

## Verification

SRC-001 through SRC-007 passed. See [verification evidence](../verification.md): local HTTP simulators, credential/failure/retry and concurrency checks, migration/restart preservation, native Windows API/storage tests, Linux non-root image/tests, 23 browser journeys and Compose regression. No live tracker credentials were supplied; no live-source success is claimed.
