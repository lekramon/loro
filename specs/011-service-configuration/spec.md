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
