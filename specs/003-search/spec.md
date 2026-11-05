# Sources and manual release search

Label: ready-for-agent
State: in-progress

## Sequence

Ticket 01 configures one Torznab-compatible source and discovers capabilities. Ticket 02 will implement manual movie release search and candidate/profile explanations. No dependency on Prowlarr/Jackett, tracker scraping, torrent retrieval, qBittorrent submission or automatic acquisition in ticket 01.

## Ticket 01 contract

- SRC-001: GET/PUT `/api/v1/source` manages one source. Unconfigured response: configured=false, revision=0, empty name/url, hasApiKey=false, capabilities=null, checkedAt=null. PUT requires onboarding, revision (0 initially), trimmed UTF-8 name 1–100 characters and exact HTTP(S) endpoint up to 2048 characters. Reject userinfo/query/fragment, missing host and invalid port. Preserve the endpoint path without appending /api. Supply API key separately: ASCII non-whitespace/non-control, up to 1024 characters; write-only in private SQLite, never logs/responses. HTTP supports self-hosted sources; HTTPS uses certificate validation. Save makes no external request.
- SRC-002: Blank/omitted apiKey preserves the existing key. clearApiKey:true explicitly clears it and cannot accompany a nonblank replacement. Endpoint changes require a nonblank replacement or explicit clear; never silently send an old key to a different endpoint. Revision compare-and-swap increments revision and clears cached capability evidence; stale/concurrent writes return 409 source-changed. Credentials and submitted URLs are omitted from errors.
- SRC-003: GET `/api/v1/source/test` performs a fresh saved-endpoint request with t=caps and optional apikey. One active test, no redirects, 2-second attempt deadline, 4 total transient network/5xx attempts with 100/200/400ms backoff within 10 seconds. Never retry auth/malformed/429. Retry-After bounded to 1–3600 seconds establishes a process-local cooldown scoped to config revision. Query credentials and upstream response bodies never enter diagnostics.
- SRC-004: XML limits: 2 MiB, depth 32, 20,000 tokens, 256 normalized categories, bounded names/known supportedParams. Reject directives/DTD, malformed/multiple documents and non-caps success. Parse search/movie-search available/supportedParams, optional limits, category/subcategory IDs and names. Missing limits remain null. Ignore server metadata, URLs and images; never fetch links. Redact key occurrences from category names. HTTP 401/403 or Torznab error code 100/101/102 means authentication failure. Classify timeout, rate limit, malformed response and unavailable separately.
- SRC-005: Successful test returns connected=true, sourceRevision, capabilities and local checkedAt; save evidence only if the revision still matches. Config changes during the call return 409 without publishing stale success. GET exposes public configuration and last successful capabilities/time. Caps success proves connectivity, not search authentication: some providers expose caps without requiring a key. Reflect absent movie/generic capabilities honestly.
- SRC-006: Additive migration 6 creates source configuration storage; preserve settings/catalog/inventory/associations and private TMDB file. Readiness validates the new table. Older binaries reject 6; rollback restores a matching pre-upgrade stopped-service backup. External outages do not affect readiness or offline catalog. Never auto-run destructive Down migrations.
- SRC-007: Settings has a localized Source form: name, endpoint, password API key, explicit clear checkbox, save/test, key-presence and last successful capability state. Clear typed secrets after save/failure; no browser persistence/prefill. Disable testing while unsaved edits exist. Show sanitized classified errors and capability-versus-authentication distinction. Preserve shared header, themes/locales. Do not expose nonfunctional release-search controls.

[Wire contract](openapi.json). Errors: 400 malformed/unknown/duplicate JSON; 403 Host/Origin/header; 413 body limit; 415 content type; 422 invalid config; 409 setup-required/source-changed/source-test-in-progress; 503 source-unconfigured/source-authentication-failed/source-unavailable/storage-unavailable; 504 source-timeout; 429 source-rate-limited with Retry-After; 502 source-invalid-response. GET works before setup; PUT requires setup. Test accepts no query or body.

## Ticket 02 boundary

Search registered movie identity using declared movie/generic query capabilities, bounded pagination/category selection, source/candidate identity and unknown-safe resolution/format/container/audio parsing. Explain the saved profile matches/mismatches without hiding unknown candidates or inventing original audio. Selection/override confirmation and durable candidate identity precede later acquisition. Publish concrete search wire/normalization contracts before implementation.

## References

[Torznab specification](https://torznab.github.io/spec-1.3-draft/torznab/Specification-v1.3.html), [Jackett interface](https://github.com/Jackett/Jackett#torznab). Caps may be public even when searches require credentials.
