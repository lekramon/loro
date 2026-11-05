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
