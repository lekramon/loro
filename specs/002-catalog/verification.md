# Ticket 01 verification — 2026-11-05

## Passed

- Production frontend build/typecheck and both OpenAPI contracts passed. Repository consistency validation covers 15 API operations and all 11 local tickets/dependency links.
- Full Windows `go test ./...` and `go vet ./...` passed during implementation. After the environment interruption made the native Go installation unavailable at its prior path, current Windows API/storage test executables were compiled with the pinned Docker Go builder and executed natively: both passed. The final Windows application executable was also cross-built successfully at ignored `bin/piratarr-catalog.exe`.
- Full Go tests and vet passed in the Linux backend build as UID/GID 10001. Linux race-detector checks `go test -race ./internal/httpapi ./internal/storage` passed.
- Public API tests use real temporary SQLite/databases/directories with TMDB/CDN HTTP simulators. They verify normalization, localized titles/original language, unknown year/runtime values, pagination, strict request validation, setup precondition, fresh cache and explicit stale fallback.
- Concurrent registration through one service and through separate service instances produces exactly one persistent TMDB identity. Repeated adds preserve internal ID, first-registration language and timestamp; provider/credential loss does not prevent returning existing records.
- Restart/offline tests retain catalog list/details and cached images. Literal title filters do not interpret percent signs as SQL wildcards. No registered title is falsely marked as associated media.
- Authentication, missing credentials, provider 404, 429/cooldown, 5xx retries, cancellation/timeout, malformed/oversized provider responses and redirects produce classified sanitized errors. Tests observe bounded attempts and no credential/upstream payload leakage.
- Artwork checks cover observed references, local URLs, no API token sent to CDN, unknown paths/sizes/traversal, invalid content, oversized images, excessive dimensions, symlink escape rejection and quota eviction.
- A real version-2 database upgrade preserves settings exactly. Injected registration failure rolls back the title; read-only storage returns 503; retry succeeds from authoritative cached details. Readiness rejects a missing required catalog table.
- All 14 Playwright journeys passed, including the existing setup/settings/mock interface flows and new localized About/Credits screen. The approved local TMDB logo renders correctly, and the required notice remains present in both locales. Reviewed `test-results/tmdb-credits.png`.
- BuildKit credential provisioning was tested using a disposable **dummy** token with a public `TMDB_BUILD_REVISION`. The dummy image reports configured=true and contains the dummy marker in its backend executable; neither API responses nor frontend assets expose it. No external provider request is made by this package check.
- A subsequent source-only image with no secret reports configured=false and contains no dummy marker, proving cache invalidation and missing-credential behavior. The host source placeholder remains empty.
- `python scripts/verify_container.py --image piratarr:catalog --port 18082` passed: non-root runtime, embedded assets, ffprobe, readiness before/after setup, path rejection, instance exclusion, restart/recreation persistence and unchanged existing media. Temporary containers and fixtures were removed.
- The opted-in live smoke check passed earlier using the privately provisioned Piratarr credential: TMDB search/details for public movie ID 27205, original-language normalization, local poster cache and successful restart/browsing/repeated registration without a credential. Existing installation data was never opened or modified by this check.
- Whitespace and Python verifier syntax checks passed. No real credential was committed or injected into the images used for package verification.

## Current environment limitations

After the host interruption, both the previously installed portable native Go toolchain and the private credential file at its documented location were unavailable. Docker supplied the pinned Go compiler for final Windows builds/tests. A repeat live check is currently unavailable because its explicit private token file is missing; this does not affect simulator coverage, package verification, readiness or offline browsing. Reprovision that private file before making new live metadata queries from a source-only development instance. No credential was reconstructed or exposed in source/logs.

## Boundaries

This ticket implements the real catalog **backend** and attribution page. The v0/mock media screens remain independent; existing-file scans, associations, acquisitions and playback are not implemented here. Ticket 02 remains open for scan/review specification.

Images tested: `piratarr:catalog` (source-only), `piratarr:catalog-test-release` (dummy credential only), and pinned Go backend images for cross-compilation/race verification. No real registered credential was embedded in these test images.
