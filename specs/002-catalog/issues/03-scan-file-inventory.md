# Scan existing movie files into a persistent inventory

Label: ready-for-agent
State: open
Blocked by: specs/002-catalog/issues/02-existing-file-scan.md

Implement SCN-001 through SCN-007 from [specification](../spec.md) and [inventory API](../library-openapi.json). Keep scanning separate from identity matching, ffprobe verification and physical import.

## Verification

Pending public API tests with real temporary SQLite/directories, migration preservation/failure, restart, repeats/missing/reappearance, concurrent exclusion, settings revision changes, symlink exclusion, limits/failures and source-file preservation. Run Windows and non-root Linux checks plus a small Files browser journey. Publish evidence before integrating.
