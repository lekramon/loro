# Observed evidence and remaining checks

## Initial diagnosis — 2026-11-06

At initial inspection, the preview on port 18085 used source kind 1337x, revision 3, origin https://1337x.st/, with no successful saved capability check. The reported source-challenge-required means the attempted challenge was unresolved. The native helper then discarded library diagnostics and no persisted .log files were found in either inspected preview data directory. Historical challenge steps cannot be reconstructed from the current logs; new structured evidence is required before claiming clearance.

Read-only Radarr API inspection found version 6.4.4.10685, six acquisition profiles, thirty quality definitions, six custom formats, four Prowlarr Torznab indexers, one qBittorrent client and one Letterboxd watchlist import. Custom formats include disc/3D exclusions, Open Matte and audio-language preferences. No passwords, API keys, private list URLs or regex bodies are recorded here. No Radarr settings were modified.

The existing native-source ticket remains uncommitted on feature/003-04-native-1337x; main's latest published repair is 05bcd81. Docker relocation could not yet be verified at E:\DockerData, and its engine probe was unresponsive. Final Linux checks are unavailable until the engine/data move is healthy. No new public-source test or download was triggered by this research.

## New expansion verification

Specifications and contracts are prepared; implementation and required checks must be recorded per ticket. Do not treat intended phases as completed features.

## Phase-zero Windows evidence — 2026-11-07

Implemented communication recorder/read API and the bottom-left drawer on the current native-source feature branch. The runtime preview was stopped while idle (zero acquisitions/downloads), and four database/credential files were copied and SHA-256 verified at E:\LouroPreview\backups\pre-communications-20261107-002706. The final tested executable is bin/louro-diagnostics.exe, serving the existing data at E:\LouroPreview\config on localhost:18085. No media or Radarr/Prowlarr configuration was changed.

- Production frontend build and all seven OpenAPI lints passed. Specification verification passed: 79 Markdown files, 38 API operations, 34 tickets and 15 foundation criteria.
- Windows go test -p 2 ./... passed with LOURO_TEST_BROWSER=1 and the installed E:\LouroBrowser runtime. This includes public-API native setup/search/selection/preparation and simulated challenge progression. go vet -p 2 ./... and go mod verify passed; the Windows executable built successfully.
- Recorder/transport/API checks cover bounded concurrent history, rotation, restart/session cursors, correlation filters, invalid input, degraded persistence, slow-start visibility, four separate source attempts, timeout codes, and secret canaries absent from public/persisted events. Browser stderr tests discard unknown/oversized/library output and report early launch failure/cleanup.
- All 34 Playwright journeys passed against the embedded UI with disposable data. New coverage includes open-only polling, pause/resume, filtering, source response privacy, both languages/themes, mobile fit and library availability. Desktop dark and mobile light screenshots were visually inspected.
- Three Python parser fixtures passed. The diagnostics changes do not submit downloads; durable submission/reconciliation/download regression journeys continue to pass.

One live source test at approximately 00:27 São Paulo time used the unchanged direct 1337x source, revision 3. The browser launched, navigated, received an HTTP 403 challenge, detected one widget and recorded seven interaction attempts. It did not emit challenge cleared or a successful parser result. The final API response was HTTP 502 / source-challenge-required, followed by browser cleanup; persistence was available. This proves trace visibility and truthful failure handling, not successful Cloudflare clearance. No repeated public-source attempts or live downloads were made.

At this phase-zero checkpoint, Docker info still timed out and E:\DockerData was absent. Final non-root Linux image/browser/Compose verification and branch publication were pending; the final gate below supersedes this historical status. Later settings/library/profile/list/subtitle phases had not started.

## Final diagnostic checks — 2026-11-07

The additional local persistent-challenge browser fixture passed in 54.5 seconds: challenge detected and widget interactions occurred, with no clearance/parser/completion event, followed by source-challenge-required and cleanup. The public API also passed a failed-log-directory check: the source test still succeeded, memory history remained readable, storageAvailable was false and the unrelated sentinel file was preserved. Worker outcome logging now follows durable result recording; the full Windows Go tests/vet passed after this adjustment. Linux amd64 cross-compilation passed; this is compilation evidence only, not the missing Linux runtime/Compose gate.

Docker Desktop's supported status command reports stopped; docker-desktop WSL is stopped. Its roughly 58 GiB VHD remains under C:\Users\ramon\AppData\Local\Docker\wsl\disk, and no Docker directory was found at E:\. The disk had not changed since 23:14 on 2026-11-06; a relocation is not currently evidenced. C: has under 1 GiB free. Desktop automation still fails to initialize with a kernel-assets/path-not-found error. No Docker disk, volume or image was deleted/reset/moved by these diagnostic probes.

The final tested preview build was started on the same port after the worker-log adjustment. Readiness returned HTTP 200; all 32 browser events, including the live challenge failure, were restored with persistence available. The final Linux amd64 executable also cross-compiled successfully; no Linux runtime success is claimed.

The offline backup's inherited ACL permits other authenticated local users to read it. Restricting this credential-containing backup to the current account was rejected by automatic approval review because access-control changes need specific user authorization. A permission request is pending; permissions have not been changed, and the verified backup is preserved.

Docker's supported restart failed with a startup deadline, and current monitor/backend logs confirmed the recurring inaccessible dockerInference socket. At 00:40 São Paulo time, Docker processes were stopped and only the verified transient IPC directory was renamed to C:\Users\ramon\AppData\Local\Docker\run.stale-20261107-004055; it was preserved rather than deleted. Docker Desktop was relaunched hidden. No VHD, images, containers or volumes were moved/reset/deleted. Engine/storage relocation and Linux acceptance still require verification.

## Storage steering and recovered engine — 2026-11-07

After the preserved IPC-directory restart, Docker info succeeded with engine 28.3.2 and Lôro readiness remained HTTP 200. Docker's engine blocker is resolved; its roughly 58 GiB disk is still on C:, E:\DockerData remains absent, and C: has about 0.5 GiB free. Large Linux image/Compose checks remain held until the approved supported disk relocation creates adequate storage; do not claim those checks passed.

The user asked for .gradle, .cache and .codex data to live on E:. Read-only measurement found .cache 2.23 GiB, .codex 0.91 GiB and no current .gradle directory. Gradle and Go cache locations were persistently configured on E: with previous values saved for rollback. E:\DevData\Move-DevData.ps1 and README.md provide an offline migration, compatibility junctions, SHA-256 verification and preserved rollback copies. Disposable cross-volume fixtures passed copy/hash, private rollback permissions, junction verification, absent-directory handling and idempotent retry. The live Codex directories remain on C: because their databases/runtimes are active; the script requires quitting Codex before applying. These machine-maintenance files are outside the project repository.

The backup-ACL question has not received specific approval: the user steered to cache storage instead. The earlier E:\LouroPreview backup permissions remain unchanged; no rejected access-control action was retried.

## Final native-source ticket gate — 2026-11-07

- `docker build -t louro:native-1337x .` passed the production frontend build, complete non-root Go tests and vet, application build, pinned Python dependency installation, sealed-browser fetch and dependency check.
- `python scripts/verify_container.py --image louro:native-1337x --port 18086` passed the disposable Linux non-root Compose lifecycle, embedded UI, ffprobe, health before/after setup, path validation, instance exclusion, restart/recreation persistence, unchanged fixture media and cleanup.
- Fresh Windows browser/API test executables were cross-compiled from the corrected tree with the pinned Go 1.27.3 image. Outside Codex's redirected AppData sandbox, `TestInstalledBrowserWithLocalFixture` passed in 30.90 seconds and `TestInstalledNativeSourceSearchAndPreparation` passed in 23.98 seconds against `E:\LouroBrowser`. The earlier sandbox-only timeout was a verification-environment artifact.
- The direct live source attempt remains truthful failure evidence only: challenge detection and interaction were visible, but clearance was not achieved and no download was submitted. Public-site clearance is environment-dependent and is not claimed.

Linux/browser/Compose acceptance and final review are complete. Ticket publication remains pending commit, rebase onto the latest `develop`, conflict-sensitive checks, feature-branch CI, and integration into `develop`; later platform-expansion tickets remain unstarted.
