# Louro product name

Label: ready-for-agent
State: done

The user selected **Louro** on 2026-11-05, replacing the Piratarr product name.

## Contract

- NAME-001: Brand the browser title, visible English/Portuguese copy, About page and `/api/v1/about.name` as Louro. Preserve approved TMDB attribution/logo. Use Louro in package identity, public OpenAPI titles, current project/product documentation and outbound User-Agent strings. Historical verification, repository/module URLs and third-party credential-registration history remain accurate rather than being rewritten.
- NAME-002: Provide `cmd/louro`, `bin/louro.exe` and Docker's `/usr/local/bin/louro` as the current executable. Keep `cmd/piratarr` and `/usr/local/bin/piratarr` as compatibility entry points sharing one CLI implementation. Publish local image `louro:local`; preserve Compose project/service identifiers and numeric user/group identities so an existing deployment keeps its identity and mount ownership.
- NAME-003: Support `LOURO_CONTAINER`, `LOURO_DATA_DIR`, `LOURO_LISTEN`, `LOURO_WORKERS`, `LOURO_TMDB_TOKEN_FILE` as preferred bootstrap names, with existing `PIRATARR_*` values as fallbacks. Explicit CLI flags retain priority for their options. Healthcheck applies the same listener precedence. Invalid preferred values fail validation rather than falling through to an old value. No secrets appear in diagnostic text.
- NAME-004: Preserve existing default data/media directory paths, database/lock/probe filenames, browser storage/event keys, mutation header, schema 7, credentials, catalog/source/download-client settings and viewer photos/preferences. No automatic directory move, database migration, secret rotation or running-service upgrade. Both entry points continue to use the same lock and storage. The GitHub repository/module/workspace locations remain stable. See [compatibility decision](../../docs/adr/0003-louro-product-name.md).

## API compatibility

Existing API routes, JSON fields, status codes and `X-Piratarr-Request` mutation header remain unchanged. Only the About response's descriptive name changes to Louro; [catalog API](../002-catalog/openapi.json) is authoritative. Branding is independent of persisted domain identity.

## Verification

Build/typecheck and API/doc consistency; public About/title and localized branding; existing browser state/locale/profile journeys; new/old bootstrap and healthcheck precedence; both executable builds; full Go suite/vet and Windows checks; disposable non-root Linux Compose against the renamed image, including the legacy executable/instance exclusion and restart persistence. Use only disposable data; no live credentials needed.
