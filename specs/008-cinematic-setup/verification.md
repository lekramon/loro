# Verification evidence

Verified on Windows on 2026-11-04.

- Production frontend build and TypeScript checks passed.
- `go test ./...` and `go vet ./...` passed.
- New public API tests cover directory-only browsing, exclusion of application data, read-only GET behavior, explicit/idempotent default creation, invalid requests, and refusal to create defaults inside application data.
- Quality API tests use real temporary SQLite databases and directories, verify exclusions survive restart and legacy updates, and reject empty, unknown and duplicate formats without advancing the revision.
- Migration test upgrades a real version-1 database, preserving revision, timestamp, resolution and audio ranking while adding all supported formats/containers.
- Eight Playwright browser tests passed, including real server-folder navigation/cancellation, onboarding with initially checked resolutions, Remux exclusion persistence, settings conflicts, and localized sun/moon controls.
- Reviewed generated desktop light, desktop dark and mobile light screenshots. No mobile horizontal overflow. Screenshots are reproducible under ignored `test-results/`.
- OpenAPI lint and `python scripts/verify_specs.py` passed: eight API operations, valid references and dependency graph.
- Native Windows executable and CGo-free Linux amd64 executable built successfully in ignored `bin/`.
- `git diff --check` passed.

Linux container execution remains unavailable because the Docker Linux daemon is stopped. Cross-compilation does not establish Linux filesystem runtime behavior. Packaging remains a separate open foundation ticket.

The directory selector is an in-app server filesystem explorer, not a native operating-system dialog. Release/container preferences are persisted now; search behavior will consume them in its future implementation slice.
