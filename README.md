# Piratarr

A local, self-hosted movie and TV library manager with original catalog, search, acquisition, and import modules.

## Current state

The foundation implements a local Go service, persistent SQLite startup, health endpoints, and a first-run setup wizard. Users can choose directories, accepted/preferred resolutions, and ranked audio preferences, save them locally, and view them after restart. Settings editing, catalog, search, and acquisition are subsequent tickets.

- [Product and milestone scope](specs/000-product/spec.md)
- [Foundation specification](specs/001-foundation/spec.md)
- [Foundation API contract](specs/001-foundation/openapi.json)
- [Implementation roadmap](specs/README.md)
- [Agent instructions](AGENTS.md)
- [Build, run, and verification commands](docs/development.md)

The selected stack is Go, React/TypeScript/Vite, and SQLite. The first supported deployment is Linux x64 using Docker Compose, published only on the host loopback interface. Windows is a development environment.

The first usable milestone is manual movie acquisition through import. TMDB, a user-configured Torznab endpoint, and qBittorrent are the initial integrations. Credentials are not needed to develop against simulators.

## Development preparation verification

Run `python scripts/verify_specs.py` to check local documentation links, tracker dependencies, acceptance references, and the foundation contract's local references. This is a repository consistency check, not an application test or a full OpenAPI validator.
