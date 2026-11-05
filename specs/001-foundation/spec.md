# Foundation: local application, onboarding, and persistent settings

Label: ready-for-agent
State: in-progress

## Problem Statement

Piratarr needs a local application that can initialize storage, explain startup failures, and preserve explicitly chosen profile and library settings. Users must be able to configure this foundation without external credentials or network services.

## Solution

Deliver the Go application and React interface as a Linux x64 container, with native Windows development support. Embed the production UI in the executable. Initialize and migrate SQLite before serving HTTP, provide liveness/readiness, and guide the user through a first-run wizard. Persist a single default profile and local download/library paths atomically. Show a settings page after onboarding.

## User Stories

1. As an administrator, I want clear startup errors for inaccessible storage, invalid configuration, migration failure, or an already-running instance, so that I can correct the deployment.
2. As a user, I want the first page to explain which settings are required, so that I can finish setup without guessing defaults.
3. As a user, I want to select accepted resolutions and one preferred resolution, so that later searches can explain compatibility.
4. As a user, I want to rank language choices and original audio, so that later selection follows my preferences.
5. As a user, I want my download and movie directories validated, so that later file operations have usable roots.
6. As a user, I want setup and subsequent settings changes to survive restarts, so that I can continue without reconfiguration.
7. As a user, I want stale settings changes rejected, so that two tabs cannot silently overwrite each other.
8. As an administrator, I want the service exposed only on loopback, so that the initial release remains local-only.

## Implementation Decisions

Follow ADR-0001. Go packages separate HTTP handlers, foundation application services, and SQLite persistence without creating a generic repository framework. The foundation does not implement download workers; it validates their configured count for subsequent features.

### Startup and persistence

- Resolve bootstrap values from CLI, then environment, then defaults. Flags: `--data-dir`, `--listen`, `--workers`; environment: `PIRATARR_DATA_DIR`, `PIRATARR_LISTEN`, `PIRATARR_WORKERS`. Workers default to 4 and must be 1–8. Invalid supplied values fail startup.
- Native listen default is `127.0.0.1:8080`. Reject non-loopback native binding unless explicit container mode `PIRATARR_CONTAINER=true`; Compose sets this and uses `0.0.0.0:8080` internally with host-loopback publishing.
- Create only the data directory automatically. Acquire its exclusive OS lock before opening the database; fail a second instance without changing data. Graceful shutdown stops requests and closes storage/lock. Handle OS termination and a 15-second shutdown deadline.
- Bootstrap database file is `piratarr.db` within the data directory. Migration 1 creates singleton application settings with revision, profile, directories, and UTC creation/update timestamps; no external credentials are part of this feature.
- Keep settings in normalized SQL tables: singleton settings record, accepted resolution rows, ordered audio rows, and foreign-key constraints. Release format and container lists are stored as JSON columns on the singleton settings row. All settings edits are one transaction. SQL constraints plus service validation enforce singleton and uniqueness rules.
- Verify migration history against embedded migrations before applying new ones, rejecting unknown/newer applied versions. No automatic downgrade. Test failed migrations with a disposable database.
- No HTTP listener starts on startup failure. Liveness indicates a running process. Readiness checks database availability and completed initialization, not onboarding or external connectivity; an unconfigured installation is ready.

### API

The authoritative wire contract is [openapi.json](openapi.json), versioned as `/api/v1`. Return JSON and structured errors with stable code, message, and optional field errors. Use 400 for malformed JSON, duplicate keys or unknown fields; 415 for a non-JSON mutation body; 413 over 64 KiB; 422 for semantic validation; 409 for setup/revision conflicts; 503 for unavailable persistence; and 500 for unexpected failures. Reject additional JSON values after the first object.

| Operation | Behavior |
|---|---|
| GET /api/v1/health/live | 200 while the HTTP process is running. |
| GET /api/v1/health/ready | 200 if database initialization and availability checks pass; otherwise 503. |
| GET /api/v1/setup | Return completion state, supported resolutions, and language codes/names for the wizard; all resolution/format/container options are initially accepted; preferred resolution and audio are explicit. |
| POST /api/v1/setup | Validate paths/profile and commit configuration once; return settings revision 1 with 201. Repeating after completion returns 409. |
| GET /api/v1/settings | Return full current non-secret settings and revision; return 409 setup-required before initial setup. |
| PUT /api/v1/settings | Replace mutable settings only if request revision matches current revision; validate and commit atomically, increment revision by one. |

State-changing requests require `X-Piratarr-Request: 1`, JSON content type, and an allowed browser Origin when present. Permit loopback Host names (`localhost`, `127.0.0.1`, `[::1]`) with the actual listening/public port. Reject other Hosts or Origins with 403, send no cross-origin permission headers, and reject cross-origin preflights. Same-origin UI and native API clients remain supported. Reverse proxy and arbitrary hostnames are deferred. Bind protection and Host/Origin validation are both required.

### Profile validation

- Supported resolutions: `480p`, `576p`, `720p`, `1080p`, `2160p`. Accepted set is nonempty and unique; preferred resolution must belong to it. Persist accepted resolutions in ascending order; input order has no meaning.
- Audio choices are unique tokens: `original` or lowercase ISO 639-1 codes. Validate codes against a bundled ISO 639-1 catalog, returned by setup as sorted code/name pairs (English names). Preserve array order as ranking. At least one choice is required; no silent preselection. Two `original` choices or repeated codes are invalid.
- Profile edits do not mutate previously confirmed acquisition decisions when those are introduced later.

### Paths

- Setup/settings accept absolute `downloadDirectory` and `movieDirectory` paths as seen by the running application. Both directories must already exist, be accessible, and permit reads and writes; test writes use a uniquely named temporary probe removed immediately. Never delete or rename user media as part of validation.
- Resolve symlinks/reparse points and return canonical absolute paths. Reject equal or nested download/movie roots. Reject either root equal to, within, or containing the data directory. Filesystem errors produce field validation details without stack traces.
- A failed validation or transaction preserves previous settings and revision. Paths are immutable bootstrap-external configuration values only in the sense of data directory; download/movie roots remain editable in this feature. Later features must define relocation behavior before allowing changes with active work.

### UI and distribution

- Wizard: server directory explorer, optional explicit default-folder creation, and absolute-path entry; preferred resolution first with all resolution/release-format/container options initially accepted and removable; searchable language choices plus original audio; ranking controls; review/save. See [cinematic setup](../008-cinematic-setup/spec.md).
- First-run page reads setup state. Settings page loads persisted revision and handles 409 by asking the user to reload; it never silently resubmits over newer data.
- After setup, show a foundation home with saved settings and a message that catalog/acquisition features are not implemented. Never show simulated success as real downloads.
- React/Vite development server proxies `/api` to the native backend. Production assets are embedded and served at `/`; unknown API routes return JSON 404 rather than the SPA. Pin build tools and dependencies with Go sums and a frontend lockfile.
- Multi-stage container build compiles frontend and Go binary, runs as a non-root user on a maintained Debian-based runtime with CA certificates and ffprobe. Compose uses host-mounted local config and a shared `/data` tree with `/data/downloads` and `/data/media/movies`; the common mount preserves future hardlink feasibility. Document ownership setup rather than running privileged or chmodding existing media.
- Exclude runtime data and secrets from Git and Docker build context. Container restart policy is `unless-stopped`; readiness healthcheck uses the application itself (`piratarr healthcheck`) without requiring curl.

## Testing Decisions

Primary boundary: net/http public API, real temporary SQLite and filesystem roots, and no real external services. Use Go testing/httptest for API integration, Vitest/React Testing Library for meaningful form behavior, and Playwright for a small browser setup-and-edit journey. Startup/process tests verify locking, migration failure, restart persistence, and safe shutdown.

| ID | Given / when | Then |
|---|---|---|
| FND-001 | Start with a new private data directory | Database initializes, live/ready return 200, and setup is incomplete. |
| FND-002 | Storage is inaccessible, bootstrap is invalid, or a migration fails | Startup exits nonzero with a sanitized cause; no listener or workers start. |
| FND-003 | Database contains newer/unknown applied migrations | Startup refuses it without changing its records. |
| FND-004 | Another instance holds the same data directory lock | Second instance exits; first instance and database remain intact. |
| FND-005 | Submit valid paths and explicit profile | Setup returns 201 and settings revision 1; restart returns the same values. |
| FND-006 | Submit empty/duplicate resolutions, invalid preferred resolution, or invalid/duplicate audio tokens | Return 422 with field errors; preserve configuration and revision. |
| FND-007 | Submit missing, relative, inaccessible, equal, nested, or data-overlapping roots | Return 422; leave user files unchanged and remove validation probes. |
| FND-008 | Setup is submitted again, or settings are read before setup | Return 409 with setup-already-completed or setup-required, respectively. |
| FND-009 | Update settings at the current revision, then send a stale update | First edit persists with incremented revision; stale edit returns 409 without overwriting. |
| FND-010 | Persistence fails during settings update | Return 503 for unavailable storage or 500 for unexpected faults; no partial settings persist. |
| FND-011 | Readiness loses storage access or shutdown starts | Readiness returns 503 while listener remains; liveness stays 200 until listener closes. |
| FND-012 | Requests violate JSON, size, mutation-header, Host, or Origin rules | Return the defined error; no mutation occurs. |
| FND-013 | Complete onboarding and edit settings through the real browser | Required choices, rankings, validation and conflict handling work; reload retains state. |
| FND-014 | Build and run Linux x64 image with local mounts | UI/API work through host loopback, assets are embedded, healthcheck passes, and restart preserves state. |
| FND-015 | Inspect logs, build context and tracked files | Runtime data and secrets are absent; no successful integration or acquisition is implied. |

## Out of Scope

Catalog, scanning, credential configuration, metadata fetching, Torznab, downloads, job execution, imports, authentication/accounts, remote access, maintenance backup commands, and media relocation. Their defaults are accepted by the product/ADRs and will be specified before their feature implementation.

## Further Notes

Startup, onboarding, and revision-safe settings editing are implemented. Docker distribution remains an open ticket. Individual tickets carry execution state and evidence. A checksum-verified portable Go 1.27.3 toolchain is installed under the development account's local application data; see development documentation for PATH setup. Docker is installed but its Linux daemon was unavailable during native verification. Reading or validating specifications does not require running the application.
