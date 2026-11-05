# Lôro product identity

Label: ready-for-agent
State: done

The user selected **Louro** on 2026-11-05, replacing the Piratarr product name.

The user refined the displayed name to **Lôro** and supplied a parrot icon later on 2026-11-05. NAME-005 supersedes the displayed spelling in NAME-001; ASCII operational identifiers remain as specified in NAME-002–NAME-004.

## Contract

- NAME-001: Brand the browser title, visible English/Portuguese copy, About page and `/api/v1/about.name` as Louro. Preserve approved TMDB attribution/logo. Use Louro in package identity, public OpenAPI titles, current project/product documentation and outbound User-Agent strings. Historical verification, repository/module URLs and third-party credential-registration history remain accurate rather than being rewritten.
- NAME-002: Provide `cmd/louro`, `bin/louro.exe` and Docker's `/usr/local/bin/louro` as the current executable. Keep `cmd/piratarr` and `/usr/local/bin/piratarr` as compatibility entry points sharing one CLI implementation. Publish local image `louro:local`; preserve Compose project/service identifiers and numeric user/group identities so an existing deployment keeps its identity and mount ownership.
- NAME-003: Support `LOURO_CONTAINER`, `LOURO_DATA_DIR`, `LOURO_LISTEN`, `LOURO_WORKERS`, `LOURO_TMDB_TOKEN_FILE` as preferred bootstrap names, with existing `PIRATARR_*` values as fallbacks. Explicit CLI flags retain priority for their options. Healthcheck applies the same listener precedence. Invalid preferred values fail validation rather than falling through to an old value. No secrets appear in diagnostic text.
- NAME-004: Preserve existing default data/media directory paths, database/lock/probe filenames, browser storage/event keys, mutation header, schema 7, credentials, catalog/source/download-client settings and viewer photos/preferences. No automatic directory move, database migration, secret rotation or running-service upgrade. Both entry points continue to use the same lock and storage. The GitHub repository/module/workspace locations remain stable. See [compatibility decision](../../docs/adr/0003-louro-product-name.md).

## API compatibility

Existing API routes, JSON fields, status codes and `X-Piratarr-Request` mutation header remain unchanged. The About response's descriptive name is now Lôro; [catalog API](../002-catalog/openapi.json) is authoritative. Branding is independent of persisted domain identity.

## Name and supplied icon update

- NAME-005: Display **Lôro** in document title, visible English/Portuguese UI text, About/API name, current documentation/OpenAPI titles and demo poster watermarks. Keep `louro` command/package/image identifiers, `LOURO_*` variables, ASCII User-Agent tokens and all compatibility/storage identifiers intact. UTF-8 display text must survive production build and API serialization.
- NAME-006: Use the user's supplied PNG unchanged as the application icon. Commit it under `web/public/brand/parrot.png` and serve it locally for the browser favicon, the main Library navigation icon and the About identity. Preserve the artwork/background/aspect ratio; no external image calls, generation or reinterpretation. Keep viewer photos separate. Use explicit image dimensions and meaningful/decorative alt text according to context. The icon and name remain legible in both themes and the compact mobile rail, with existing shared header/PT-EN geometry preserved.
- NAME-007: Verify source/copy identity, production asset delivery/MIME and browser loading, favicon reference, accented title and About API name, both locales/themes and mobile layout. Existing preference/profile journeys must pass; no API routes, schema or runtime migration is added.

## Verification

Build/typecheck and API/doc consistency; public About/title and localized branding; existing browser state/locale/profile journeys; new/old bootstrap and healthcheck precedence; both executable builds; full Go suite/vet and Windows checks; disposable non-root Linux Compose against the renamed image, including the legacy executable/instance exclusion and restart persistence. Use only disposable data; no live credentials needed.
