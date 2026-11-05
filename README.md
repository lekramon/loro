# Piratarr

A local, self-hosted movie and TV library manager with original catalog, search, acquisition, and import modules.

## Current state

The foundation implements a local Go service, persistent SQLite startup, health endpoints, and a first-run setup wizard. Users can choose and edit directories, accepted/preferred resolutions, and ranked audio preferences. Saves persist across restart, and revision conflicts prevent one tab from overwriting another. Linux Docker Compose distribution is verified, with non-root execution, a bounded healthcheck, local mounts and settings persistence across recreation. The catalog backend now implements TMDB metadata search, duplicate-safe movie registration, local metadata/artwork caches and offline browsing. File scanning, source search and acquisition remain subsequent slices.

The interface supports light/dark themes and English (US)/Português (Brasil), selectable in the header. Display preferences are saved in the browser and do not change movie audio preferences. On first visit, language and theme follow browser defaults.

- [Product and milestone scope](specs/000-product/spec.md)
- [Foundation specification](specs/001-foundation/spec.md)
- [Foundation API contract](specs/001-foundation/openapi.json)
- [Implementation roadmap](specs/README.md)
- [Agent instructions](AGENTS.md)
- [Build, run, and verification commands](docs/development.md)
- [Linux Docker Compose deployment](docs/deployment.md)

The selected stack is Go, React/TypeScript/Vite, and SQLite. The first supported deployment is Linux x64 using Docker Compose, published only on the host loopback interface. Windows is a development environment.

The first usable milestone is manual movie acquisition through import. TMDB, a user-configured Torznab endpoint, and qBittorrent are the initial integrations. Credentials are not needed to develop against simulators.

TMDB access will use application credentials registered for Piratarr, supplied by release packaging. Setup/settings allow saving a TMDB Read Access Token locally; provisioned releases can still supply a fallback credential. The development credential is privately provisioned and verified; the metadata adapter and private-file/BuildKit release provisioning hooks are implemented. Source-only builds need an available private credential file for new live provider queries. See [maintainer registration instructions](docs/tmdb-registration.md).

## Development preparation verification

Run `python scripts/verify_specs.py` to check local documentation links, tracker dependencies, acceptance references, and the foundation contract's local references. This is a repository consistency check, not an application test or a full OpenAPI validator.

The real catalog API is described in the [catalog contract](specs/002-catalog/openapi.json) and [API examples](specs/002-catalog/api-examples.md). The `/preview` interface remains mock data while the new design is developed in v0. Provider credits are available at `/about`.

Service configuration now includes a write-only TMDB token field in setup/settings. See [configuration and design update](specs/011-service-configuration/spec.md).

Open `/search` for real TMDB movie metadata search. Setup/settings include Test connection for the saved credential; the preview still uses mock library records. Navigation stays visible on desktop and as an icon rail on mobile.

The library and settings now share one header/sidebar. Real metadata search lives inside Library (`/preview#/search`). Start with one viewer, add more explicitly, and edit viewer names/photos using the pencil overlay or Manage profiles. Viewer photos/preferences remain browser-local.

Library → Files now scans the configured movie folder into a persistent inventory. Repeat scans retain IDs, distinguish present/missing paths and preserve existing media. Movie identification and video verification follow in the next catalog ticket. See [inventory evidence](specs/002-catalog/inventory-verification.md).

Files review now supports explicit movie association, ffprobe video checks, Ignore and Reopen. Associate from registered metadata or choose/register a TMDB result, confirm identity, then verify. Changed or missing observed files lose verification on scan. Docker includes ffprobe; native development requires ffprobe on PATH.

Settings → Release source now saves a Torznab-compatible endpoint and write-only API key, with fresh capability discovery, classified errors and persisted last-success state. Library → Search now registers a TMDB movie through Find releases, and Library → Releases searches registered movies, displays normalized candidates and explains accepted, mismatching or unknown profile attributes. Download submission is the next stage. See [source specification](specs/003-search/spec.md).
