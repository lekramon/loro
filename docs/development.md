# Development

## Toolchain

Pinned versions are Go 1.27.3, Node.js 24.22.1, and pnpm 11.25.0. Dependency versions are recorded in `go.mod`, `go.sum`, `package.json`, and `pnpm-lock.yaml`. Place Go, Node.js, and pnpm on PATH. The local development setup downloaded a checksum-verified Go archive to `%LOCALAPPDATA%/PiratarrToolchains/go1.27.3/go`; add its `bin` directory to the current shell PATH if using that installation.

## Clean-checkout build

```powershell
pnpm install --frozen-lockfile
pnpm run check:api
pnpm run build
go test ./...
go build -o bin/piratarr.exe ./cmd/piratarr
```

The frontend build must precede Go build/tests because the backend embeds its generated assets. Build output is ignored by Git. On Linux, use `-o bin/piratarr` instead.

## Run

```powershell
go run ./cmd/piratarr
```

Open `http://127.0.0.1:8080` and follow **Start setup**. Supply separate existing absolute download/movie directories, choose accepted/preferred resolutions, and rank audio languages or original audio. Saving validates paths and persists one profile. Use **Edit settings** to change it; every save carries the loaded revision, and a stale tab must explicitly reload before editing again. Folder changes update configuration only and never relocate existing media. Catalog, search, and downloads belong to subsequent tickets and are unavailable.

Header controls switch between light/dark and en-US/pt-BR without resetting forms. Browser-local keys `piratarr.interface.theme` and `piratarr.interface.locale` store explicit selections; clearing site storage restores browser-based defaults. UI messages are paired and type-checked in `web/src/messages.ts`; backend protocol messages remain English and are localized when displayed. Interface language is independent of ISO audio-language tokens.

Default native data roots are `%LOCALAPPDATA%/Piratarr` on Windows and `/var/lib/piratarr` on Linux. Prefer an explicit private writable directory on Linux when not running as a service. Never use the synchronized repository for runtime data.

Bootstrap flags override environment variables:

| Flag | Environment | Default |
|---|---|---|
| `--data-dir` | `PIRATARR_DATA_DIR` | Platform-specific private directory |
| `--listen` | `PIRATARR_LISTEN` | `127.0.0.1:8080` |
| `--workers` | `PIRATARR_WORKERS` | 4 (validated, workers not implemented yet) |

`PIRATARR_CONTAINER=true` permits non-loopback binding and changes the default data root to `/config`. Only enable it in an isolated container with a host-loopback-only published port. Container packaging is implemented and verified in ticket 04; see the deployment guide.

For frontend development, keep the native backend running and run `pnpm dev` in another terminal. Open `http://127.0.0.1:5173`. The loopback Vite server proxies API requests to the backend at port 8080, rewriting browser Origin only in development. Production uses one same-origin endpoint.

## Checks

```powershell
pnpm run typecheck
pnpm run check:api
go vet ./...
go test ./...
python scripts/verify_specs.py
pnpm run test:browser
```

Go tests use temporary local directories, real SQLite, and child processes for instance exclusion and process-death recovery. They do not use real external credentials. Shut down with Ctrl+C. Startup errors are structured JSON on stderr and stop the process before an HTTP listener is opened.

Browser tests launch their own temporary-data backend on port 18080 and require Chromium installed with `pnpm exec playwright install chromium`. Keep that port available. Playwright stores screenshots and other results in ignored `test-results/`.

To cross-build for the first deployment target:

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -o bin/piratarr-linux-amd64 ./cmd/piratarr
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

Cross-compilation does not substitute for Linux runtime and filesystem tests. Ticket 04 verified the Linux container build, non-root backend tests and disposable Compose acceptance suite; rerun the deployment checks when packaging or filesystem behavior changes.


## Setup folders and quality

Setup includes an in-app directory explorer that lists folders on the server, including mounted container paths. It does not open the browser client's native OS dialog. Absolute paths remain editable. Application-private data and files are excluded from browsing.

The explicit **Create and use default folders** action creates `~/Videos/Piratarr/downloads` and `~/Videos/Piratarr/movies` on native installations; in containers it uses `/data/downloads` and `/data/movies`. Mount `/data` to durable host storage when using this option. This action does not save settings or complete onboarding. Existing folders and files are preserved; if creation fails after creating one directory, that empty directory may remain for retry.

New setups accept every supported resolution, release format and container. Select a preferred resolution and remove unwanted options. Remux is a release format, while MKV and MP4 are containers. Existing saved resolution choices remain unchanged during migration. Format/container preferences are persisted now; candidate filtering will use these fields when manual search is implemented.


## Mock media prototype

Open `/preview` in the native app or Vite dev server. Setup is not required. Choose a demo viewer to explore home, movies, search, favorites, details, simulated playback/downloads, and viewer editing. The Library/Prototype link in real settings also opens it.

Preview navigation uses hash routes, for example `/preview#/movies`. All catalog fixtures and original SVG artwork are bundled locally under `web/src/mockData.ts` and `web/public/mock-art/`. No external metadata, movie images, video or download requests are made.

Viewer names, active viewer, favorites and watched flags persist under the browser-local key `piratarr.demo.viewers.v1`. Delete only that key in browser developer tools to reset the sample profiles. Download pause/resume state and the simulated player's controls are in-memory demonstrations and reset on reload. Blocked storage falls back to in-memory profiles.

Real setup and settings remain at `/setup` and `/settings`. The prototype never saves configuration, scans files or submits downloads. Viewer profiles are distinct from acquisition quality preferences and provide no access control.


## Container distribution

See [Linux deployment](deployment.md) for pinned builds, local mounts, healthcheck and disposable container verification. Packaging and Linux runtime verification passed in foundation ticket 04.


## Catalog backend

The real catalog API is independent of the mock `/preview` interface. See [catalog API examples](../specs/002-catalog/api-examples.md) and the [wire contract](../specs/002-catalog/openapi.json). Metadata search and movie registration use the backend-only application credential; list/details and previously cached images remain available without it. Registration requires saved setup, uses authoritative TMDB details, and is idempotent per TMDB ID.

`--tmdb-token-file` overrides `PIRATARR_TMDB_TOKEN_FILE`; the optional absolute private-file path is a maintainer/development bootstrap option. No token is requested during onboarding. Read [credential provisioning/rotation](tmdb-registration.md) for the optional BuildKit release-secret hook and opt-in live test. The source placeholder `internal/tmdb/release-credential.txt` must stay empty in Git. Store no real token in it.

Schema migration 3 adds durable movies, bounded metadata query caches and observed artwork references. Artwork bytes live under the private data root's `cache/tmdb/` directory and are capped at 128 MiB. Search caches last 24 hours; stale provider-error fallbacks are explicitly marked. Registration does not scan folders or mark any media file available.

The frontend design remains under development in v0. Do not interpret a mock movie or simulated download as an actual catalog/acquisition record. `/about` is the implemented attribution screen.

Package verification commands: `python scripts/verify_metadata_package.py --image piratarr:catalog` and, for an image built with the known dummy verification token only, `python scripts/verify_metadata_package.py --image piratarr:catalog-test-release --dummy-provisioned`. These start and remove only temporary containers and do not contact TMDB. Never use the dummy-provisioned check to infer that a real application credential authenticates successfully.

## Local service metadata configuration

Open `/setup` (or `/settings` after onboarding), enter a TMDB API Read Access Token under Movie metadata, then select Save TMDB token. Saving is independent of folder/profile settings and applies without restart. Leave the input blank to preserve the saved credential. The service writes a private `credentials/tmdb-read-token` inside its data directory; include it in future secret-bearing configuration backups. It overrides the bootstrap file and bundled fallback. Status reports presence only; search verifies provider acceptance. No read-back or browser storage of tokens.

For Windows testing when native Go is unavailable, set `PIRATARR_TEST_EXECUTABLE` to an absolute path of a freshly built Windows executable before `pnpm run test:browser`. The browser harness still creates disposable application storage.

Use **Test connection** in Movie metadata to make a fresh authenticated provider search; it does not use the local search cache. Save edited token input before testing. Open `/search` from the persistent sidebar to search real metadata by movie title and optional year. The `/preview` search continues to filter mock records.

## Unified library and viewer photos

Library navigation now contains Search at `/preview#/search`; `/search` redirects there. Primary navigation contains Library and Settings. One shared header controls theme/language across every section. Fresh browser storage starts with one Ramon viewer; choose Manage profiles or its pencil overlay to edit name/photo, or add a viewer explicitly. Select JPEG/PNG/WebP up to 5 MB; the browser crops/resizes to a 256px JPEG and stores it with the viewer locally. Cancel preserves the saved photo; Remove photo takes effect on Save profile. Photos are not uploaded or shared between browsers/devices. Storage failure displays a warning. Untouched former demo seeds are omitted on migration; customized viewers are preserved.

## Existing-file inventory

Open `/preview#/files`, then Scan folder. Setup must configure a movie directory first. This screen uses real inventory, independently of the mocked Movies screen; no TMDB credential is needed. It scans only that movie folder, skips symlinks and unsupported extensions, and never changes source media. Inventory presence is not proof of movie identity or a valid video stream.

Scans are manual and synchronous, with one concurrent scan, a cooperative 10-second deadline, depth 64, 100,000 directory entries and 10,000 supported files. Failed/interrupted scans retain the last committed snapshot. After an uncertain response, refresh inventory or repeat the scan safely. Missing paths remain in history; successful rescans restore their IDs when present again. Changing folders keeps previous-root records and lists only the current folder.

Schema 4 adds library inventory/summary tables and preserves catalog/settings. Startup applies it automatically. Older binaries refuse the newer database; return to an older build by restoring its matching pre-upgrade stopped-service data directory copy. Do not invoke destructive SQL Down migrations. Runtime data and backup copies belong outside this checkout.

## Existing-file association and review

In Library → Files, select Associate for a present file. Search Registered movies, or switch to TMDB and explicitly choose a result to register its metadata. Confirm that the selected movie matches this file, then Verify and associate. Metadata registration and file association are separate actions; canceling after choosing a TMDB result leaves its registered metadata but does not link the file.

The service opens the file under the configured root and feeds that handle to ffprobe with pipe-only protocols, an eight-second timeout and bounded JSON output. Docker supplies ffprobe; native Windows development must install ffmpeg and make ffprobe available on the service process PATH. If missing, review returns video-probe-unavailable without changing records. Video-stream detection proves technical validity, not movie identity.

Ignore clears any existing link and persists across scans; Reopen returns ignored entries to pending. Unchanged links survive scans/restart; changed size/mtime or an observed missing file clears verification. Rescanning also advances file revisions, so stale review forms must refresh. API catalog hasMedia reflects associated present files in the current root, based on last observations. This is not continuous integrity monitoring or content hashing.

Schema 5 adds review fields to inventory while preserving existing records. Older binaries reject it; rollback requires restoring a matching pre-upgrade stopped-service data copy. No media file is moved, renamed or deleted by review.

## Torznab source configuration

After onboarding, open Settings → Release source. Enter a descriptive name, the exact Torznab API endpoint without query parameters, and its API key separately. Use an address reachable from the service: localhost in Docker refers to that container; host.docker.internal can address a host service on Docker Desktop. Saving only persists configuration. Test source makes a fresh t=caps request and stores normalized search/category capabilities on success. No Jackett/Prowlarr installation is required by the adapter; provide any compatible endpoint.

Blank key preserves the saved value. To switch endpoints, provide a new key or explicitly select Clear saved API key. Keys never return through the API or persist in browser storage; they reside in private SQLite, so data-directory backups contain credentials. A capability endpoint may be public even when searches require authentication; success is not proof of search access. Last successful discovery remains visible with its timestamp after a test failure.

Migration 6 adds the source configuration table while preserving existing catalog/settings/associations. Older binaries reject the upgraded DB; rollback requires a matching stopped-service data copy. No live endpoint is bundled. Manual release search uses these saved capabilities.

## Manual release search

Open Library → Search, find a TMDB movie, then choose Find releases. This registers metadata idempotently and opens Library → Releases. Select a category if needed and Search releases. Releases also lists and filters registered movies; those searches use saved movie identity and work without contacting TMDB. Source requests still require the configured endpoint to be available.

Candidates remain in provider order. Each displays known/unknown resolution, release format, container and observed audio languages, plus the saved profile values and match/mismatch/unknown checks. Preferred resolution is marked separately. Subtitle languages, MULTI/DUAL and audio codecs do not establish audio language. Source claims do not prove movie identity. Unknowns and mismatches are visible; no downloads or profile changes occur here.

Searches prefer declared TMDB movie-ID queries, then declared movie text queries, then generic text search. Test the source after saving or changing its endpoint. Pagination uses bounded provider offsets; opaque result IDs are observations scoped to source revision, not durable acquisitions or verified torrent hashes. The acquisition stage must persist selection, confirm unknown/mismatching profile overrides and establish torrent identity before any submission.

Example after setup and source testing (PowerShell):

```powershell
$base = 'http://127.0.0.1:18084/api/v1'
$movies = Invoke-RestMethod "$base/catalog/movies?limit=25&offset=0"
$movieId = $movies.items[0].id
Invoke-RestMethod "$base/releases?movieId=$movieId&limit=25&offset=0"
```

No schema change accompanies release search; source configuration still uses schema 6. The preview executable for this slice is `bin/piratarr-releases.exe`. Start it from the repository root with the same external data directory used by your previous preview; stop that preview process first. Do not run two instances against the same data directory.

## qBittorrent connection preparation

After onboarding, open Settings → Download client · qBittorrent. Supply the WebUI base URL, username and password, then Save download client and Test download client. Use the base URL rather than `/api/v2`; a reverse-proxy prefix is supported. In Docker, localhost addresses that container; Docker Desktop can reach host services through host.docker.internal when configured accordingly. HTTP supports local services and HTTPS validates certificates.

A blank password preserves the existing password only when URL and username stay the same. Initial configuration and changing either field require a replacement password. Password spaces are preserved. Passwords are write-only local SQLite data, so backups contain secrets. They are never prefilled or saved in browser storage. Sessions are private to each test; the backend logs in, reads app/WebAPI versions, then attempts bounded logout. It does not retry logins or change torrents, categories, tags or client settings. Standard credential login with a usable SID cookie is required; connection testing is not validation of download permission or shared-folder mapping.

Last successful versions/time remain visible after a failed test; saving configuration clears them. Client outages do not affect local readiness. Revision conflicts require Reload download client. The next acquisition ticket must define durable selection, override confirmation, torrent identity and reconciliation before adding submission controls. Existing mock Downloads remains simulated.

Migration 7 adds download-client configuration and preserves settings/catalog/associations/source configuration. Older binaries reject schema 7. Before upgrading real data, retain a matching stopped-service data-directory backup for rollback; do not run SQL Down migrations. The Windows preview executable for this slice is `bin/piratarr-client.exe`, image `piratarr:client`. Use your existing external data directory and stop its previous service first.

Public API example (after saving through Settings):

```powershell
$base = 'http://127.0.0.1:18084/api/v1'
Invoke-RestMethod "$base/download-client"
Invoke-RestMethod "$base/download-client/test" -Method Post -Headers @{ 'X-Piratarr-Request' = '1' } -ContentType 'application/json' -Body '{}'
```

Live validation requires your own accessible qBittorrent WebUI credentials. Disposable HTTP simulators cover development without them.

## PT/EN display toggle

The shared header uses a rounded PT/EN selector: PT is Brazilian Portuguese and EN is US English. Click either segment or use Tab with Space/Enter. The active segment is inset, with contrast adapted to the theme. Display choice keeps its existing browser-local storage key and does not change the media profile. The sun/moon buttons retain their behavior.

The combined preview with qBittorrent connection settings and this control is `bin/piratarr-language.exe`, Linux image `piratarr:language`. Restart with the same external data directory; the connection feature uses schema 7 and this presentation change adds no migration.
