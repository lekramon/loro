# Verification — 2026-11-05

## Passed

- Production build and TypeScript checking (`pnpm run build`). All three OpenAPI contracts pass Redocly lint (`pnpm run check:api`); repository references/dependencies pass `python scripts/verify_specs.py`.
- Full Go regression tests run as UID/GID 10001 in the pinned Go 1.27.3 Linux builder. `go vet ./...` passes. Windows API test executable cross-built with that compiler and executed natively: PASS.
- Public API simulator checks cover saving without onboarding, missing bootstrap file, provider use of stored credentials, replacement, malformed/duplicate/extra/empty tokens, body limit, mutation header, foreign Origin, sanitized persistence failures and no secret returned. Existing catalog tests remain passing.
- Application startup/shutdown/reopen checks pass on Linux and Windows: service-local credential survives application restart despite missing bootstrap file. Linux credential permissions are 0600. Windows uses the existing private-directory ACL mechanism.
- All 15 browser journeys pass, including setup/settings, locales/themes, directory browsing, revision conflicts, mock navigation/profiles and attribution. New token journey checks password masking, explicit save, blank field preservation, replacement, reload presence and absence from localStorage/status responses.
- Reviewed desktop dark mock home and light mobile service configuration screenshots. Mobile page has no horizontal overflow; navigation, cyan accents and flat surfaces follow the Jellyfin reference. Existing mock illustrations remain intentional placeholders.
- After the full suite passed, the focused token journey passed again with corrected screenshot scroll position; no product changes followed.

## Boundaries

Native Go SDK is unavailable; Docker provided the pinned compiler and Windows cross-build. No live TMDB authentication was attempted and no real credential was entered, retrieved, logged or committed. Token presence does not prove TMDB acceptance. The service is still local-only. Preview content remains mocked; this ticket does not connect real catalog records to that interface.

Local executable: ignored `bin/piratarr-config.exe`. Browser runs used disposable data directories; the existing user preview was not stopped or modified. Disposable builder container removed.


## Ticket 02 — connection diagnostics and live search

- Existing running service was inspected through public API only: configured=true; Inception returned 12 results. A subsequent new Perfect Days (2023) query returned one result with cache.hit=false and cache.stale=false, confirming current live access. No credentials were read, printed or changed.
- Full Go tests and vet passed using pinned Go 1.27.3 in Docker. Non-root UID/GID 10001 regression tests passed. Current HTTP API tests were cross-compiled and executed natively on Windows: PASS.
- New public API simulator tests prove connection tests bypass fresh local search caches on repeated requests and use the effective saved credential. Missing/rejected tokens, timeout, rate limit with Retry-After, provider failure and malformed response produce sanitized errors.
- All 17 browser journeys passed, including existing onboarding/settings/themes/locales/mock flows. New journeys verify saved-token testing, unsaved-edit guard, Portuguese rejected-token feedback, real search request parameters, unknown years, pagination preserving submitted criteria, stale-cache indication and error clearing of old results.
- After changing only the preview global search link, the six affected mock/search journeys passed again against the final embedded executable. This verifies that the global icon navigates to `/search` and mock library filters remain available.
- Production TypeScript/Vite build, three OpenAPI lints and repository link/dependency validation passed. Reviewed desktop mock home and desktop/mobile live-search screenshots: top branding removed, sidebar remains visible, mobile rail has no page overflow. About retains TMDB attribution with the same persistent navigation.
- Final ignored Windows executable: `bin/piratarr-metadata.exe`. Native Go SDK remains unavailable; Docker supplied the compiler. Existing user service was not stopped or upgraded; browser checks used disposable data roots. Live checks used the already running service's search operation, not the new test endpoint; that endpoint is covered by controlled simulator tests.
