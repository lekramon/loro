# Service configuration and Jellyfin layout

Label: ready-for-agent
State: done

## Contract

User instruction supersedes mandatory maintainer-only TMDB provisioning. Setup and saved settings expose a separate Metadata section to save or replace a TMDB API Read Access Token, without a terminal command. Saving is explicit and independent of folder/profile saves. Empty input never changes a credential. Token is write-only, stored atomically with private permissions under the service data directory, read on subsequent requests and after restart. Saved service token overrides CLI/environment file and bundled credentials. No read-back, localStorage, logging or token in status. Status indicates presence, not successful authentication. Syntax validation does not imply provider acceptance. Setup can finish without metadata access.

PUT `/api/v1/metadata/credential` accepts exactly `{ "token": "..." }`, 1–8192 ASCII bearer characters (letters, digits, dot, underscore, hyphen), surrounding whitespace trimmed. Reject malformed/duplicate/extra fields (400), empty/invalid value (422), unsupported service or write failure/stopping (503). Return only `{ "provider":"tmdb", "configured":true }` (200). Existing Host/Origin, JSON, bounded body and mutation-header protections apply. GET existing metadata/status remains unchanged. No deletion operation in this ticket; blank fields preserve existing credentials.

Use Jellyfin official home/library screenshots as design reference: https://jellyfin.org/ . Original implementation with Piratarr branding. Flat neutral surfaces, cyan accents, compact top header with hamburger navigation, centered tabs, horizontal library and poster rows, compact profile chooser and a simple settings page. Keep English/Portuguese, light/dark, keyboard focus, mobile layouts and explicitly labeled mock data. No real catalog UI, acquisition, authentication or playback added.

## Acceptance

- CFG-001: Save/replace through public API and form; reload/restart retains presence and provider calls use the saved value, even if a bootstrap file is missing.
- CFG-002: Responses and browser storage contain no credential; invalid payload and persistence failures preserve existing credential; cross-origin and missing mutation header rejected.
- CFG-003: Setup/settings expose localized password input, configured/unconfigured status, explicit save and sanitized errors; clearing input does not overwrite.
- CFG-004: Setup/settings and preview have matching flat Jellyfin-inspired visual language at desktop/mobile in both themes; existing journeys remain usable.

See [API contract](openapi.json) and [ticket](issues/01-service-configuration.md).


## Connection test, live search and persistent navigation update

Ticket 02 supersedes CFG-004's hidden drawer: remove top branding from setup/settings, live search and preview; keep a persistent left sidebar on desktop and an always-visible icon rail on mobile. Existing preview drawer remains an optional secondary mock-navigation aid.

- CFG-005: GET `/api/v1/metadata/test` performs an uncached authenticated movie search against TMDB, using the saved effective credential with existing bounded retries/timeouts. Returns `{ "provider":"tmdb", "connected":true }` only after a valid provider response. No secret or remote payload returned. Missing/rejected credentials 503; unavailable 503; timeout 504; malformed 502; rate-limited 429 with Retry-After. Existing Host/Origin protections apply. No credential modification or onboarding required. Configured status continues to mean presence only.
- CFG-006: A localized Test connection button tests the saved token, shows pending/success/actionable errors, and blocks concurrent save/test. Unsaved input explicitly requires saving first. Saving clears previous test feedback and says saved, not authenticated.
- CFG-007: `/search` provides real movie metadata search through the existing backend, with localized search title/year controls, result cards/year/overview, pagination, loading/empty/errors and stale-cache indication. Images use local URLs and credits link to About. New searches clear old results; use interface locale; no browser/provider token or direct external calls. No registration, acquisition or playback added. Clear links distinguish live search from mock preview. The global preview search icon opens `/search`; filters inside the mock library and its secondary drawer search remain mock-only.
- CFG-008: Persistent navigation is visible at desktop and mobile widths, with accessible links to preview, live search and configuration. No top Piratarr branding; existing theme/locale controls, mock flows and keyboard navigation preserved.

[Ticket 02](issues/02-connection-test-and-navigation.md) records execution and evidence. Provider application authentication follows https://developer.themoviedb.org/docs/authentication-application .


## Unified library and viewer identity — ticket 03

Supersedes separate library header/drawer and top-level search navigation. One shared header and persistent sidebar serve setup/settings, About and every library screen. Primary navigation contains Library and Settings/Setup. Inside Library, secondary navigation contains Home, Movies, Search, Favorites, Downloads and viewer selection. Real TMDB search is `/preview#/search`; `/search` remains a redirect compatibility entry. It works without choosing a viewer; metadata is not mocked. Existing movie filters remain mock-library filters. Same header DOM/CSS, utility order, heights and spacing everywhere; no top branding, no secondary hamburger/drawer. Keep themes/locales and mobile rail; use flat Jellyfin-inspired surfaces with modest typography and fewer dividers.

- UI-001: Fresh browser storage starts with one Ramon viewer; additional viewers require user creation. On migration remove only unchanged former Cinema/Family shipped seeds, preserve customized viewers and favorite/watched/name state. Profiles remain browser-local prototype identities, not accounts. Preserve in-memory use when storage fails.
- UI-002: Netflix-inspired centered chooser with square avatars, Manage profiles mode and pencil overlay on hover/focus/edit mode. Edit name/photo by explicit save, cancel discards. Photo chooser accepts JPEG/PNG/WebP up to 5 MiB, decodes/validates pixels (max 8192 per side), center-crops to 256 square and re-encodes JPEG at bounded quality. Save only generated JPEG data URL (max 256 KiB) in existing local profile state. Reject SVG/invalid/oversized files with localized feedback; remove photo restores initials. Never upload image or retain original file/metadata. CSP permits data images only in img-src; other directives unchanged.
- UI-003: Selected photo appears on chooser and the shared header/settings identity. Add profiles remains available (max 12), with isolated favorites/watched state. Keyboard and touch edit action accessible without hover. Storage failures show a visible warning rather than claiming durable save.
- UI-004: All sections use the same header utility component. Library navigation and search stay within the same shell; search is nested under Library, never a separate primary entry. No duplicate main navigation drawer. At desktop and mobile widths utilities have identical bounding boxes across library/settings. Existing acquisition configuration, token test, About attribution and mock behavior preserved.

No JSON API fields or SQLite migrations change. Existing [catalog API](../002-catalog/openapi.json) supplies real search; [service credential contract](openapi.json) remains unchanged. [Ticket 03](issues/03-profile-photos-and-unified-shell.md) records evidence. Visual references: [Jellyfin home](https://jellyfin.org/assets/images/10.8-home-4a73a92bf90d1eeffa5081201ca9c7bb.png) and [library](https://jellyfin.org/assets/images/10.8-library-9c07ba46877a03e749c0c8b5ed157e53.png). No upstream assets/source copied.
