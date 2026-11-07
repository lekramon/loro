# Direct 1337x adapter with a managed local browser

Label: ready-for-agent
State: done
Blocked by: specs/003-search/issues/03-search-timeout.md

Implement NAT-001–NAT-006 in the [specification](../spec.md) and [API contract](../openapi.json).

## Acceptance

- Select Torznab or direct 1337x in Settings, preserving the existing configured source until explicitly saved. Default existing installations to Torznab.
- Search and page movie candidates directly, without Prowlarr/Jackett/Byparr services; use a managed local browser dependency for challenges.
- Resolve only the selected candidate's same-origin details page to a magnet; retain profile confirmation, v1 identity validation, durable selection and submission rules.
- Bound browser work, isolate temporary sessions, cancel the entire child-process tree, redact diagnostics, and show missing runtime/unresolved challenge errors.
- Package the Linux browser runtime and provide a pinned native Windows installer outside the synchronized checkout.
- Verify public API, stored-source migration/preservation, parser failures, browser cancellation, candidate selection/preparation, and localized UI with simulators. Report live site results separately.

## Expanded phase-zero acceptance

The user expanded this current ticket on 2026-11-06 to make source failures diagnosable. Implement OBS-001–OBS-008 in the [platform specification](../../013-platform-expansion/spec.md) and its [read API](../../013-platform-expansion/openapi.json) before closing this ticket: sanitized browser phases, bounded communication history and the bottom-left viewer across existing app screens. This extends the same current ticket/branch; the settings/library ticket remains next after verification/integration.

## Verification

Final verification complete.

- Full Linux non-root Go tests/vet passed during implementation, including schema-11 preservation/failure/newer-reader refusal and source/acquisition regressions. Final runtime checks must be repeated for the final browser helper.
- Windows public API source/release suites and the installed-browser source setup → search → selection → identity-preparation journey passed with disposable SQLite/directories and HTTP fixtures. No live download was submitted.
- Windows browser fixtures passed streaming documents, Unicode titles, simulated challenge recovery, rejection of results ending on another origin, missing runtime, cancellation and process-tree cleanup. HTTP redirects may be contacted before final-origin rejection; browser-engine redirect blocking was removed because it interfered with live verification.
- Five Playwright source/release journeys passed, including direct-source settings and en-US/pt-BR diagnostics. Three Python parser fixtures passed, including actual category paths and 40 raw row positions.
- A live native 1337x movie search returned 40 movie candidates without Prowlarr/Byparr. Other attempts returned an unresolved challenge; success is not a universal clearance guarantee.
- After relocation to `E:\Projetos\piratarr`, frozen dependency installation, frontend build, six OpenAPI lints, specification verification and whitespace checks passed. The pnpm store is on E:; 1,758 non-node_modules files were verified by SHA-256 and Git HEAD/branch/status matched before removing the old project files.

Publication remains: commit the completed ticket, rebase the unpublished feature commit onto the latest `develop`, repeat conflict-sensitive checks, push, wait for feature-branch CI, and integrate into `develop`. Docker's metadata writes previously failed after C: filled and its filesystem remounted read-only. The engine is reachable again; final Linux and installed-browser validation pass.

## Comments

Claimed by Codex on 2026-11-06 after the user chose replacement of Prowlarr with direct adapters. First adapter: the user's configured protected 1337x source. Additional sites require their own adapters; this is not a general Cardigann engine. No downloads are authorized by this task.


Recovery note: main contains the published timeout/cancellation repair at 05bcd81. Native-adapter work is preserved on feature/003-04-native-1337x, uncommitted. The original repository directory is an empty placeholder for the current chat's working directory; use E:\Projetos\piratarr explicitly. Desktop automation could not initialize because kernel assets could not be written (path not found), so the user is completing the approved Docker settings move manually. The old native preview has not been upgraded to schema 11 by this task.

## Phase-zero diagnostics update — 2026-11-07

See [current verification](../../013-platform-expansion/verification.md) for implemented communication history, privacy/restart/rotation checks, full Windows Go/vet/module checks and all 34 passing browser journeys. The idle E: preview has an offline hash-verified backup and now runs louro-diagnostics.exe on localhost:18085. A single live attempt detected the challenge/widget and seven interactions but returned source-challenge-required; clearance was not achieved. Docker's move/engine remains unverified, so final Linux checks and publication are still blocked. The ticket remains in progress; later phase tickets remain open.

Historical storage note: Docker engine 28.3.2 recovered after preserving the stale IPC directory. At that point its disk remained on C: with about 0.5 GiB free and final Linux checks awaited the authorized move to E:\DockerData. The final gate below supersedes that check status; no integration into main has occurred.

## Final gate attempt — 2026-11-07

- `docker build -t louro:native-1337x .` passed. The pinned image build completed the production frontend build, full non-root Go tests and vet, application build, Python dependency lock installation, browser fetch and dependency check.
- `python scripts/verify_container.py --image louro:native-1337x --port 18086` passed the disposable Linux non-root Compose lifecycle, embedded UI, ffprobe, health before/after setup, path validation, instance exclusion, restart/recreation persistence, unchanged fixture media and cleanup.
- Fresh Windows browser and API test executables were cross-compiled from the corrected working tree with the pinned Go 1.27.3 image. The first sandboxed runs timed out because Codex redirected AppData/browser-cache state into an isolated package environment; this was a verification-environment artifact, not a product regression. Outside that sandbox, `TestInstalledBrowserWithLocalFixture` passed in 30.90 seconds and `TestInstalledNativeSourceSearchAndPreparation` passed in 23.98 seconds against `E:\LouroBrowser` after the final parser/module-boundary corrections.
- Final review corrections reject structurally empty result tables, inject source-reference resolution through the acquisition service boundary, synchronize verification evidence, remove generated Python cache and use the Lôro display name. Three parser fixtures, the focused acquisition/source/API suites, full non-root `go test -p 2 ./...` and `go vet -p 2 ./...`, frontend production build, seven OpenAPI lints, specification verification and `git diff --check` passed.
