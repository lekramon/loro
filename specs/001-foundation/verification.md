# Foundation verification ledger

State: partially implemented

Tickets 01–03 are implemented and verified on Windows, including Linux cross-builds. Container distribution remains unimplemented. The following table distinguishes delivered behavior from future acceptance work.

| Criteria | Primary ticket | Evidence |
|---|---|---|
| FND-001–FND-004 | 01 | Passed native/API/real storage/process tests; Linux runtime pending ticket 04 |
| FND-005–FND-008 | 02 | Passed real API/storage restart, profile/path, repetition, alias, and Windows ACL tests |
| FND-009 | 03 | Passed atomic revision increment, stale rejection, controlled concurrent updates, and restart tests |
| FND-010 | 02–03 | Setup and settings-update rollback/read-only fault tests pass; failed writes preserve revision and every profile row |
| FND-011 | 01 | Passed readiness/storage-loss/shutdown and liveness checks |
| FND-012 | 01–03 | Host/Origin, header/content-type, duplicate/unknown/malformed/trailing JSON, update revisions, and body limits pass |
| FND-013 | 02–03 | Browser onboarding/editing, ranking, field feedback, save/reload, cancellation, and setup/edit conflict recovery pass |
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

## Ticket 02 implementation evidence — 2026-11-04

- Go API integration tests passed with actual SQLite transactions and filesystem validation. Saved profile values and revision survive closing and reopening storage. Concurrent setup requests cannot create mixed or duplicate profiles.
- Strict JSON validation rejects duplicate/unknown keys (including incorrect casing), malformed data, trailing values, invalid types, and oversized known-length/chunked bodies. Semantic errors return field-specific 422 responses without persistence.
- Windows read-only ACL and directory symlink tests passed, including alias-overlap rejection. Existing media and roots remain unchanged; temporary validation probes are removed.
- A trigger failing the last table write proves full rollback through the public API; a read-only writer produces 503, and recovery succeeds after clearing the fault.
- Three Chromium tests passed against the built frontend and real Go service: setup entry/required fields, complete onboarding and another tab's conflict, and API JSON 404 behavior. Saved audio order and directory contents were verified.
- Frontend build/typecheck, pinned OpenAPI validation, Go static analysis, Windows build, and CGO-free Linux x64 cross-build passed. Linux runtime checks remain pending ticket 04; the Unix mode-bit permission test is prepared but was not executed on this Windows host.

## Ticket 03 implementation evidence — 2026-11-04

- All six foundation API operations are implemented. PUT settings requires the loaded revision; service validation and the write transaction both check it. A test barrier forces both competing requests past the initial read before writing, proving transaction-level exclusion with one 200 and one 409.
- Real SQLite trigger and read-only failures prove settings updates are atomic across revision, timestamp, directories, resolutions, and ordered audio choices. Correcting the fault permits a single successful retry. Invalid requests preserve the original state.
- API tests verify persisted updates after storage restart, profile normalization, directory changes without moving existing files, strict update JSON, blocked bootstrap-only settings, and stale updates without overwrites.
- The Chromium journey verifies prefilled editing, changed quality/audio ranking and directory roots, validation errors without revision changes, second-tab conflict, explicit reload, cancellation, and preserved media. All three browser tests passed.
- Go tests, static analysis, frontend build/typecheck, OpenAPI validation, and Windows/Linux builds passed. The Docker Linux engine remained unavailable; Linux runtime/distribution verification is still required by ticket 04.


## Ticket 04 preparation evidence — 2026-11-05

State: implementation prepared; Linux acceptance verification pending.

- Added a non-root multi-stage Dockerfile with pinned Go/Node/Debian manifest digests, pnpm version, exact ffmpeg/CA package versions and immutable Debian snapshot indexes. Actual image build has not completed because the Linux engine is unavailable.
- Added Compose with host-loopback-only publishing, explicit existing bind mounts, a common `/data` tree, bounded readiness healthcheck, graceful shutdown and restart policy. Standalone Docker Compose v2.38.2 validated `compose.yaml` successfully without a running engine.
- Added `piratarr healthcheck`: bounded loopback-only HTTP readiness check, no database/instance lock, no proxy or redirect following. New tests cover ready/unavailable/malformed/redirect responses, option validation, non-local rejection and context deadlines.
- `go test ./...`, `go vet ./...`, native executable build, specification consistency and diff whitespace checks passed. The new executable successfully probed the running native mock preview on port 8081 while its database remained locked by that service.
- Prepared `scripts/verify_container.py` for real Compose API/volume verification using only fresh temporary directories. Python syntax validation passed; the container acceptance script has not run.
- Resolved image digests from the official registry and ffmpeg/CA package pins from Debian-security's 2026-11-04 snapshot. The main snapshot index request timed out during preparation; apt resolution/install remains part of the pending actual image build.
- Docker Desktop was started through its CLI and application executable, then exited before its Linux engine could answer. The user confirmed that Docker Desktop displays an error; its exact text is pending. WSL status reports version 2.5.9.0 and WSL 2 as default. No WSL reset, distribution deletion, system feature changes or Docker data cleanup was attempted.

FND-014/FND-015 and Linux rechecks of FND-005/FND-007 are not marked passed. Keep this work on its feature branch until the image build and runtime acceptance checks succeed. See [deployment instructions](../../docs/deployment.md).


## Ticket 04 completed verification — 2026-11-05

Supersedes the pending preparation status above. FND-014 and FND-015 passed, with FND-005/FND-007 rechecked through the real Linux distribution.

- Recovered Docker Desktop startup by preserving only its two stale zero-byte runtime socket objects in `%LOCALAPPDATA%/Docker/run.stale-20261105-004801` and allowing a fresh `run` directory. The engine now responds as Linux Docker 28.3.2. No factory reset, WSL reset, image/volume prune or persistent data removal was used. The targeted workaround is described in [Docker issue 554](https://github.com/docker/desktop-feedback/issues/554).
- `docker build --platform linux/amd64 -t piratarr:local .` completed with pinned base digests, exact runtime packages and snapshot indexes. Frontend build/typecheck, full Go tests and Go vet ran successfully inside the build; Go checks run as UID/GID 10001 on the Linux filesystem.
- Runtime image includes ffmpeg/ffprobe 7:7.1.5-0+deb13u1 and CA certificates 20250419+deb13u1. Snapshot download errors were handled with bounded whole-install retries, disabled HTTP pipelining and a BuildKit package cache; repository signatures and package checksums remain enforced. No TLS or signature checks are disabled.
- Actual Docker context export contained 80 source/build-input files and no runtime data, credentials, node_modules, Git history or generated assets. Temporary audit output was verified and removed.
- `python scripts/verify_container.py --image piratarr:local --port 18082` passed against the production image through Docker Compose with real disposable bind mounts and SQLite. Port 8080 was occupied by the user's native preview, which remained running; configurable `PIRATARR_PORT` matches the container listener and host-loopback publishing.
- Verified actual UID 10001, private config mode 0700, ffprobe availability, CA bundle presence and absence of Node/npm. Embedded HTML, referenced JavaScript/CSS and local artwork were fetched successfully without a frontend server.
- Readiness and the CLI healthcheck succeeded before and after onboarding. Missing/nested/equal/data-overlapping/read-only and symlink-alias roots were rejected without saving setup. A second instance was refused specifically by the data-directory lock.
- Saved directories, profile, revision and timestamp remained identical after restart and force-recreation. Existing media checksum, mode and ownership remained identical; no temporary validation probes were left behind.
- The verifier stopped its unique Compose project and removed only its verified temporary fixture. No acceptance containers remained running afterward.
- Standalone/native Go tests, Go vet, healthcheck smoke verification, Compose configuration checks, Python syntax validation, documentation consistency and whitespace checks passed during this ticket. No real integration credentials were needed or accessed.

Validated image ID: `sha256:61ef308cabcbe2027f33f04c89081663bc2b4668d87f91d53d0f110b566d1ef5` (Linux amd64). Environment: Docker Desktop Linux engine on Windows, including Linux-native build/test temporary files and disposable host bind mounts.

The running native preview holds `bin/piratarr.exe` open, so the refreshed Windows executable is available side by side as `bin/piratarr-next.exe`; its readiness probe against the existing preview on port 8080 passed. The running process was not stopped or replaced.
