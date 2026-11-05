# ADR-0001: Local modular application

Status: accepted by the user on 2026-11-04.

## Decision

Use Go with standard `net/http`, React/TypeScript/Vite, and SQLite in a modular monolith. Serve the embedded production frontend and JSON REST API from the same process. Keep responsibilities for catalog, library, sources, search, profiles, acquisitions, downloads, import, jobs, and history separate. Modules communicate through Go interfaces and application services; avoid a generic plugin engine.

Use `database/sql` with `modernc.org/sqlite` and explicit SQL. Configure foreign keys, WAL, and a 5,000 ms busy timeout on every connection. Use one write connection and a bounded read pool. Hold write transactions only for local state changes, never network requests or file copying.

Embed ordered Goose SQL migrations. Before serving HTTP or running workers, reject schema versions or applied migration histories unsupported by this binary, apply pending migrations, and validate required configuration. A migration failure aborts startup. Acquire an exclusive OS-level data-directory lock first, released by the OS after process death; a second instance using that directory fails startup. Never delete a lock file to bypass a live instance.

Develop on Windows and verify Linux filesystem semantics in containers. The initial supported distribution is Linux x64 with Docker Compose. The service listens on port 8080; native execution binds 127.0.0.1 and the container binds 0.0.0.0, with Compose publishing `127.0.0.1:8080:8080`. No account management or remote access in v1. Validate Host and browser Origin headers; disable cross-origin access. Localhost access is not protection from other processes running as the same OS user.

Use a private data directory outside the checkout and synchronized folders. Linux default: `/var/lib/piratarr`; Windows development default: `%LOCALAPPDATA%/Piratarr`. Container default: `/config`. Configuration precedence is explicit CLI flag, then `PIRATARR_*` environment variable, then platform default for bootstrap options (data directory, listener, worker count). User configuration and profile live in SQLite. Bootstrap options are not writable through the API.

Database and private directories use owner-only permissions on Linux and a user-restricted ACL on Windows. Secrets are local and write-only through the API; there is no claim of encryption against the host account. Credential backups must be treated as secrets. Application logs are structured JSON, redact credentials, and omit full integration URLs/query strings. Retain operational history for 90 days; preserve unresolved review records until resolved.

## Integrations

- TMDB API v3 with a user-provided read token. Cache metadata and images locally; expose attribution in About and metadata-derived screens. Include TMDB's approved logo and required notice, with current terms checked before release.
- Native Torznab endpoint adapter: capabilities, movie search, pagination, and normalized candidates. No dependency on another indexer manager. An actual endpoint and credentials are user configuration, not a bundled tracker.
- qBittorrent 5.x WebUI API: dedicated `piratarr` category and per-acquisition tags. External download engine remains outside the application.
- `ffprobe` in the runtime image for video-stream validation. Development tests use a controlled process substitute plus one real fixture check when the binary is available.

These choices are documented in [TMDB guides](https://developer.themoviedb.org/docs/getting-started), [Torznab guidance](https://github.com/Jackett/Jackett#torznab), [qBittorrent API](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-%28qBittorrent-5.0%29), [SQLite driver docs](https://pkg.go.dev/modernc.org/sqlite), and [Goose docs](https://pressly.github.io/goose/).

## Consequences

One local database and application process simplify installation. SQLite write coordination and migration discipline are explicit responsibilities. Native credentials and real integration checks are unnecessary for simulator-based development. Toolchains and dependencies are pinned in application manifests and lockfiles; development documentation lists verified build commands and platform limitations.

## Maintenance

An explicit `piratarr backup --output <archive>` command acquires the same exclusive directory lock while the service is stopped, checkpoints/closes SQLite, and archives the database plus a versioned manifest containing app/schema versions and configuration. Exclude media and reconstructible caches. The archive contains credentials. `piratarr restore --input <archive> --data-dir <empty-directory>` requires an empty destination, rejects unsafe archive paths and unsupported newer schemas, and removes partial restoration on failure. Never overwrite existing data. Schema downgrades are unsupported; return to a previous release by restoring its matching backup.
