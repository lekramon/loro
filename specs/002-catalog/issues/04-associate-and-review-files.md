# Associate observed files and review ambiguous identity

Label: ready-for-agent
State: done
Blocked by: specs/002-catalog/issues/03-scan-file-inventory.md

Implement ASC-001 through ASC-007 from [specification](../spec.md) and [API contract](../library-openapi.json). Preserve physical media and keep explicit identity confirmation separate from technical ffprobe validation.

## Comments

Claimed by Codex on 2026-11-05. Branch feature/04-associate-and-review-files. Use lean-build and migration skills; one bounded association/review slice.

## Verification

ASC-001 through ASC-007 passed. See [verification evidence](../review-verification.md): Linux/non-root and Windows API/storage/process tests, real MKV/MP4 probes in the runtime container, 22 browser journeys, migration preservation/rollback and Compose regression. Native ffprobe is absent and explicitly not claimed as a passed native check.
