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
