# Scan existing movie files into a persistent inventory

Label: ready-for-agent
State: done
Blocked by: specs/002-catalog/issues/02-existing-file-scan.md

Implement SCN-001 through SCN-007 from [specification](../spec.md) and [inventory API](../library-openapi.json). Keep scanning separate from identity matching, ffprobe verification and physical import.

## Verification

SCN-001 through SCN-007 passed. See [verification evidence](../inventory-verification.md): Linux non-root tests/vet/image, native Windows API/storage tests, 21 browser journeys, Compose regression, migration preservation/rollback-on-failure, restart, missing/reappearance, exclusion, limits, symlinks and unchanged source media. Association/review remains ticket 04.

## Comments

Claimed by Codex on 2026-11-05 after publishing ticket 02.
