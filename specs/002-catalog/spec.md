# Movie catalog and metadata

Label: ready-for-agent
State: in-progress

## Scope and sequence

Ticket 01 implements backend TMDB search, durable duplicate-safe registration, offline catalog browsing, local artwork caching and attribution. Ticket 02 subsequently specifies existing-file scanning and explicit association/review. The interface is being designed independently in v0; the mock UI remains separate from actual catalog records.

## Ticket 01 contract

- Search TMDB API v3 by query, optional year, page and interface metadata language (`en-US` or `pt-BR`, default en-US). Normalize stable TMDB identity, title/original title, original language, release year, overview, genres, runtime and locally served poster/backdrop URLs. Unknown year/runtime remain null, missing text/artwork stays empty; no invented quality/audio availability.
- Public operations: GET `/api/v1/metadata/status`, GET `/api/v1/metadata/search`, POST/GET `/api/v1/catalog/movies`, GET `/api/v1/catalog/movies/{id}`, GET `/api/v1/metadata/images/{size}/{filename}`, GET `/api/v1/about`. Wire schemas are in [openapi.json](openapi.json). Existing Host/Origin/mutation protections and 64 KiB body limits apply. Catalog mutations require completed onboarding. Reading an empty catalog does not require onboarding.
- Registration body contains only positive `tmdbId` and optional `language`. Fetch authoritative movie details rather than trusting browser-supplied metadata. A unique TMDB database constraint protects identity; first registration returns 201, repeat returns 200 and the same internal ID without requiring provider access. A registered title has `hasMedia=false` until later file associations exist; this does not prove that no file exists on disk.
- List catalog through bounded `limit` (1–100, default 50), `offset` (0–1000000) and literal title filter `q` (at most 200 characters); deterministic title/ID ordering. Individual lookup uses internal ID. Catalog reads never call TMDB and survive provider failure, missing credentials and restart.
- Cache normalized metadata search/detail responses in SQLite for 24 hours, at most 500 cached queries. Fresh responses are served without network. Expired cached responses may be returned as explicitly stale when provider access fails; never turn a provider error into an empty successful search. Registered metadata remains durable and is not evicted with query caches. Registration may use a cached authoritative detail response.
- Only artwork paths observed in provider metadata may be fetched, using fixed TMDB CDN sizes w342/w780 and a fixed HTTPS origin. Return local API URLs. Fetch lazily with a maximum 8 MiB image, JPEG/PNG validation and bounded pixel dimensions. Store verified images atomically under private application cache directories; cap artwork cache at 128 MiB by evicting oldest managed files. Cached images remain available offline until eviction. Reject traversal, unknown sizes, unobserved image paths, redirects and non-image responses. Never forward the API token to the image CDN.
- External requests: at most four concurrent HTTP operations, two-second per-attempt timeout and an overall request budget below the server write deadline. Retry transient network errors and 5xx at most three times after the first attempt, with bounded exponential backoff. Never retry authentication, malformed responses, 404 or 429 blindly. Honor bounded Retry-After via a process-local provider cooldown.
- Classify missing credentials (503), authentication rejection (503), unavailable provider (503), timeout (504), rate limit (429 with Retry-After), missing provider movie (404), malformed provider responses (502), storage unavailability (503), invalid input (422), malformed JSON (400), missing setup (409), unknown catalog ID (404) and unexpected persistence failure (500). Public errors/logs never include credentials, provider response bodies or full integration URLs.
- Runtime credential hook: `--tmdb-token-file` overrides `PIRATARR_TMDB_TOKEN_FILE`. Without a file, use an optional application credential embedded by maintainer release packaging. Source-only builds have no credential by default. Private files are read afresh for new provider calls, enabling rotation; absent/unreadable/invalid credentials disable metadata queries without preventing startup, readiness or offline browsing. Status reports configured presence, not successful authentication, and exposes no credential/path.
- Release hook: BuildKit secret `tmdb_token` populates an otherwise empty embedded backend credential during compilation. Its value never appears in command arguments, logs, frontend assets or tracked source. A non-secret `TMDB_BUILD_REVISION` build argument invalidates secret-insensitive build cache for provisioning/rotation. Bundled application credentials are recoverable from a distributed binary, as accepted in ADR-0001. Test this hook with a dummy credential; use the privately provisioned token only in an explicitly opted-in live smoke check.
- Provide an About/Credits screen with an approved locally bundled TMDB logo and the exact required notice. Metadata/catalog responses link to the attribution resource. No redesign of the v0/mock UI is part of this ticket.

## Acceptance

| ID | Scenario | Observable result |
|---|---|---|
| CAT-001 | Search metadata with simulator | Stable normalized IDs, original language, correct pagination and local artwork URLs; unknown values remain unknown |
| CAT-002 | Register repeatedly/concurrently, including during provider outage | One durable title with the same internal ID, original metadata preserved |
| CAT-003 | Restart and browse offline | Registered list/details and previously cached artwork remain available |
| CAT-004 | Provider auth/timeout/rate-limit/5xx/malformed response | Classified sanitized error, bounded attempts, cooldown respected; no partial title registration |
| CAT-005 | Cache hit/expired cache with failing provider | Fresh cache avoids calls; stale fallback is explicitly marked and retains original data |
| CAT-006 | Artwork request is unsafe, redirected, oversized or invalid | Request rejected; no token forwarded or file written outside managed cache |
| CAT-007 | Existing foundation DB upgrades or persistence fails | Settings preserved; failed writes do not create partial/duplicate catalog state |
| CAT-008 | Credentials are absent, rejected, rotated or provided by release hook | Startup/offline browsing continue; secret values never appear in responses or frontend |
| CAT-009 | Browse attribution | Approved local logo and required notice available in About/Credits |

Primary verification uses public HTTP APIs, real temporary SQLite/databases/directories and controlled TMDB/CDN simulators. Validate on Windows and Linux containers; a small browser About journey verifies attribution. Real provider checks use temporary catalog storage and never mutate the user's existing installation.

## References

[Search](https://developer.themoviedb.org/reference/search-movie), [details](https://developer.themoviedb.org/reference/movie-details), [images](https://developer.themoviedb.org/docs/image-basics), [rate limiting](https://developer.themoviedb.org/docs/rate-limiting), [attribution](https://developer.themoviedb.org/docs/faq).
