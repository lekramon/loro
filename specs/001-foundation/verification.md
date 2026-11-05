# Foundation verification ledger

State: partially implemented

Ticket 01 is implemented and verified on Windows, including a Linux cross-build. Setup, settings editing, and container distribution remain unimplemented. The following table distinguishes delivered behavior from future acceptance work.

| Criteria | Primary ticket | Evidence |
|---|---|---|
| FND-001–FND-004 | 01 | Passed native/API/real storage/process tests; Linux runtime pending ticket 04 |
| FND-005–FND-008 | 02 | Pending implementation |
| FND-009 | 03 | Pending implementation |
| FND-010 | 02–03 | Pending implementation |
| FND-011 | 01 | Passed readiness/storage-loss/shutdown and liveness checks |
| FND-012 | 01–03 | Host/Origin, header/content-type, and known-length body limits pass; JSON decoding pending 02 |
| FND-013 | 02–03 | Startup preview/navigation/reload pass; onboarding and editing pending |
| FND-014 | 04 | Pending implementation |
| FND-015 | 01–04 | Preview is explicit; runtime data/build outputs ignored; distribution checks pending |

Implementation evidence must include Windows/native results, Linux container results, database and directory isolation, and any skipped checks with their reasons. Startup refusal scenarios must use disposable databases and preserve all user data.

Documentation preparation is separately checked by `python scripts/verify_specs.py`. It checks repository consistency only; it is not evidence that foundation behavior works.

## Historical preparation snapshot — before ticket 01

- `python scripts/verify_specs.py`: passed; 16 Markdown files, six API operations, four tickets, and 15 foundation acceptance criteria checked. Local links, contract references, and ticket graph are consistent.
- The OpenAPI document parses as JSON. A complete OpenAPI validator is not installed; full standards validation remains part of contract/tooling setup in ticket 01.
- `git diff --check`: no tracked diff whitespace errors. These artifacts are currently untracked, so this command alone does not validate their contents.
- Go is not on this shell's PATH. No Go application, browser journey, container build, or real integration checks were run for this preparation delivery.

## Ticket 01 implementation evidence — 2026-11-04

- Go 1.27.3 portable installation is available outside the checkout. Native tests, static analysis, and executable build passed; Linux amd64 cross-build passed with CGO disabled.
- React/TypeScript/Vite production build and typecheck passed. Two Chromium browser tests passed, including the actual embedded frontend served by the Go application.
- Pinned Redocly OpenAPI validation passed. The contract identifies the two mutation operations that are planned rather than implemented.
- Database tests prove startup/refusal, transactional failed migration rollback, pragma application on four independent read connections, and restart persistence. Process tests prove exclusion and recovery after a forced kill.
- Damaged required schema is refused at startup even when its migration version is recognized. Startup/shutdown failure paths release the lock and listener.
- Linux runtime/container checks were not run: Docker is installed, but its Linux engine pipe was unavailable. No external credentials were required or accessed.
- Full acceptance and commands are recorded in ticket 01; build/run instructions are in `docs/development.md`.
