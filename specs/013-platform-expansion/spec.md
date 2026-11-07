# Unified settings, communications, media libraries and subtitles

Label: ready-for-agent
State: in-progress

## Scope and authority

The user requested this expansion on 2026-11-06: inspect the current Cloudflare failure; show communications from a bottom-left control; expand Settings like Radarr; provide Movies, Series and Anime library sections; prepare Letterboxd/IMDb list integrations; and integrate native subtitle management equivalent to Bazarr. Specifications precede implementation. This document extends, rather than discards, the movie-first foundation, file-preservation rules and original-module architecture.

[Research](research.md) maps current Radarr/Bazarr behavior and the historical Reddit guide. [Observed local evidence](verification.md) is separate from these requirements. [Phase-zero API](openapi.json) is authoritative only for communications; later phases must publish their own endpoint/migration amendments before implementation. No new product answers are required for these defaults.

## Delivery order

| Phase | Executable ticket | Outcome and boundary |
|---|---|---|
| 0 | [Existing native-source ticket](../003-search/issues/04-native-1337x.md) | Finish native-source work and the communications foundation below; add browser phase evidence and the bottom-left log viewer. The user expanded this current ticket's diagnostics acceptance; keep it on its existing feature branch until checks pass. |
| 1 | [Settings and library navigation](issues/01-settings-and-library-navigation.md) | Expandable Settings tree, working routes to existing controls, Movies/Series/Anime navigation, honest planned states. |
| 2 | [Acquisition policy](issues/02-acquisition-policy.md) | Named acquisition profiles, quality size definitions and custom-format scoring, with explicit movie assignments and conservative unknowns. |
| 3 | [Imports and operational gate](issues/03-import-and-operations-gate.md) | Specify/finish verified file import and backup/restore before new automated writers. Preserve existing milestone 005/006 ownership. |
| 4 | [Series and anime](issues/04-series-and-anime.md) | Real series/seasons/episodes and anime classification; manual acquisition before monitoring/pack automation. |
| 5 | [Import lists](issues/05-import-lists.md) | CSV/export first, source-specific adapters later; preview, deduplication and explicit catalog additions. |
| 6 | [Subtitles](issues/06-native-subtitles.md) | Owned subtitle assets per movie/episode file, local/provider search and verified sidecar download; optional automatic policies follow manual validation. |
| 7 | [Monitoring and upgrades](issues/07-opt-in-automation.md) | Explicit opt-in monitoring, upgrades and subtitle synchronization after recovery/file guarantees pass. |

Finish, verify, commit, push and fast-forward each ticket into main before starting the next. The current native source branch remains incomplete until its Windows/Linux checks pass. Future tickets labeled needs-triage need their concrete wire contracts and migration designs before claiming; the roadmap alone is not implementation evidence.

## Phase 0: communication diagnostics

- OBS-001: Record application-owned API requests and outbound TMDB (including artwork), Torznab, selected torrent-reference retrieval, qBittorrent connection/submission/reconciliation/polling and native-browser operations. Record every outbound attempt separately, including retries. This is application diagnostics, not a packet capture of other programs. Record start and response/completion events so long requests are visible while running.
- OBS-002: Public events have a stable event ID, timestamp, correlation ID, integration, operation, phase, severity, nullable HTTP status and elapsed milliseconds, nullable bounded retry-after seconds, sanitized error code and numeric counts where relevant. Store fixed operation/phase names instead of raw request URLs. Worker operations get correlations too. Request/response summaries must make status, timeouts, cooldown and challenge outcomes inspectable.
- OBS-003: Never retain/expose credentials, authorization/cookies, submitted tokens, usernames/passwords, provider URL query strings, torrent URLs/magnets/passkeys, raw request/response bodies, full HTML, browser fingerprints or private filesystem paths. Construct events from an allowlisted schema; do not attempt to redact an arbitrary upstream error dump. Unknown exceptions become classified codes. Render text without HTML. Log readers never call an external integration.
- OBS-004: Native browser phases include launch, navigation, document response, challenge detected, widget found/interaction attempted, challenge cleared, parser result, timeout/error and cleanup. Widget interaction is evidence of an attempt, never proof of clearance. Clearance requires a non-challenge document plus valid source parsing. An unresolved challenge returns source-challenge-required; it does not claim source authentication/capability success. No unbounded retry or human-verification guarantee.
- OBS-005: Keep a bounded in-memory window of 2,000 events and private rotating NDJSON files under the data directory, at most three files of 2 MiB each and seven days of retained history. Reconstruct recent history after restart; ignore invalid/truncated records safely. Never fail an acquisition because log persistence failed. Report degraded log storage visibly and retain the bounded memory window. Do not put logs in the repository. Routine health/log polling must not create a self-sustaining logging loop.
- OBS-006: GET /api/v1/system/communications is read-only and follows the existing Host/Origin restrictions. Return ordered events, a cursor scoped to the recorder session, reset/truncation information and persistence availability. Limit 1–200 (default 100); filters: known integration and optional validated correlation ID. Reject unknown/duplicate parameters and malformed cursors. On a stale/restarted cursor return a reset page rather than silently losing diagnostics. No arbitrary log path or unredacted file-serving endpoint.
- OBS-007: A persistent Logs button at the bottom-left opens an accessible communication drawer on setup, settings and library screens. Poll only while open, allow pause/resume and integration filtering, preserve scroll position unless following new events, and distinguish empty, loading, disconnected and degraded-persistence states. Pausing display does not stop recording. Show correlation, timestamps, phase, HTTP status/error and elapsed time in en-US/pt-BR, with consistent themes and responsive layout. Do not log the viewer's own polling.
- OBS-008: Test through public APIs with real temporary data directories and external HTTP simulators. Put secret canaries in URL/userinfo/query/headers/body/upstream errors; assert they are absent from the API, persisted logs and UI. Verify slow-start visibility, error/response status, retry separation, cursor reset, rotation bounds, persistence failure, restart history, browser challenge progression and no duplicate acquisition effects. Windows and non-root Linux checks remain required.

## Phase 1: settings and library navigation

- NAV-001: Settings is an expandable group with Media Management, Profiles, Quality, Custom Formats, Indexers, Download Clients, Import Lists, Subtitles, General and System. Show the active child and accessible expanded state; default expanded inside Settings. Keep the existing persistent sidebar and shared theme/language/profile header. Preserve direct /settings and /setup links and setup return behavior. System is operational information, not a quality setting.
- NAV-002: Route existing directory controls to Media Management, the current acquisition profile to Profiles, source configuration/test to Indexers, qBittorrent to Download Clients, metadata credential/test and application preferences to General, and existing readiness/configuration summaries plus communications to System. Reuse the existing revision-safe settings APIs. Section edits preserve fields owned by other sections; stale revisions remain conflicts.
- NAV-003: Quality means size definitions for quality classes, Profiles means accepted/ranked acquisition policy, and Custom Formats means additional release traits and their profile-specific scores. Until those richer backends exist, render explanatory planned states without fake Save/Test buttons or claimed persistence. Current resolution/format/container/audio preferences continue to work under Profiles.
- NAV-004: Add Series and Anime beside Movies. Anime is a classification of a movie or series, not a third incompatible content identity. Anime items appear in the Anime view; filters may also expose their underlying type. Preserve existing movie IDs and routes. Until phase 4, series/anime views clearly state that catalog support is planned, without fabricated live records or requests to nonexistent endpoints.
- NAV-005: Browser journeys cover all settings routes, return navigation, current forms, keyboard expansion, log button visibility, both locales/themes and narrow layouts. No external request starts merely by opening a section.

## Phase 2: acquisition policy

- POL-001: Introduce named acquisition profiles, distinct from viewer profiles and subtitle profiles. Migrate the current singleton into a named default without changing accepted values or past acquisition snapshots. Assign a profile to each title; changing defaults does not retroactively rewrite earlier choices. Continue initial accepted-all resolution choices and the preferred-resolution selection already agreed.
- POL-002: Quality definitions combine explicit resolution/source evidence and optional minimum/preferred/maximum size per runtime minute. Missing runtime/size is unknown. Named profiles choose accepted qualities, ordering, cutoff and future upgrade permission. No automatic upgrade is enabled by introducing these settings.
- POL-003: Custom formats are original bounded structured predicates over known release evidence, with profile-specific scores and minimum/cutoff scores. Initial examples include 3D, disc images, Open Matte, codecs/HDR and explicit audio-language evidence. Use Go-compatible bounded matching; reject unsupported imported regex constructs visibly. Do not copy guide regex/JSON as product logic. A title token is not proof of an audio track.
- POL-004: Explanations show matched rules, unknown evidence, points and profile revision. Manual overrides still require confirmation without modifying the profile. Publish backward-compatible API/schema amendments and migration/rollback evidence before implementation.

## Phase 3: imports and operations prerequisite

Use the existing import/recovery specification boundaries for hardlink/copy, ffprobe verification, conflict review, confirmed replacement, preserved downloads and restart journals. Finish stopped-service backup/restore and Linux/Windows preservation checks before adding automatic sidecar writers or list/upgrade effects. This document does not mark unimplemented imports or maintenance commands complete.

## Phase 4: series and anime

- MED-001: Series identity, seasons, episodes, specials and media-file associations remain distinct. TMDB movie and television IDs cannot collide. Preserve movie APIs/IDs; publish additive series/episode contracts and migration checks first.
- MED-002: Anime classification is explicit metadata/user choice; never infer it solely from Japanese audio, country or animation genre. Support season/episode and optional absolute numbering; uncertain numbering and multiple-episode associations enter review. Anime movies retain movie identity.
- MED-003: Ship catalog and manual episode search/acquisition first. Season packs, daily numbering, multi-episode import and monitoring require their own tested contracts before enablement. No misleading automatic series controls during the movie-only stage.

## Phase 5: list ingestion

- LST-001: Prepare adapters for user-exported Letterboxd and IMDb CSV, followed by supported provider APIs/feeds when available. Provider access approval, credentials, quotas and current API terms are external dependencies. Do not depend on the Reddit guide's third-party RSS bridge or promise unsupported IMDb custom-list scraping.
- LST-002: Bound file/input size, validate rows and identifiers, match metadata and preview additions/duplicates/ambiguities before committing. Refresh is idempotent by provider/list/title identity. Do not automatically search/download on list ingestion by default.
- LST-003: Removing a title from a remote list never removes local titles, media or subtitles. Missing/ambiguous matches require review. Store list provenance locally; credentials are write-only and private list URLs are omitted from logs. Scheduled sync is opt-in with persisted jobs and bounded retries.

## Phase 6: native subtitles

- SUB-001: Implement subtitle coordination inside Lôro; no required Bazarr service. Provider adapters may use external services such as OpenSubtitles with locally stored write-only credentials and documented account/API prerequisites. Development uses simulators until real credentials are available.
- SUB-002: A subtitle profile specifies ordered languages (initial pt-BR default, user-editable), full/forced/SDH preferences and minimum matching evidence. Display subtitle status/search on each movie and each series episode, and summarize missing/present states on the series page. Do not attach one subtitle file to an entire series.
- SUB-003: Subtitle assets belong to a concrete validated media-file revision and carry provider identity, language/flags, match evidence, checksum and history. Embedded tracks and external sidecars are distinct. A replaced/changed video invalidates old matching evidence without silently deleting user subtitles.
- SUB-004: Start with manual local subtitle detection, provider search and explicit download of SRT/VTT. Check media existence, identity, language, bounded size/encoding/content and destination ownership. Stage and verify alongside media before publication, e.g. Title.pt-BR.srt or Title.pt-BR.forced.srt. Never modify video/download contents or overwrite an existing sidecar silently. Retry/cancellation/restart reconciliation must avoid duplicate writes.
- SUB-005: Automatic missing-subtitle search, improvements and synchronization follow explicit opt-in policies after manual correctness. Synchronization creates a reviewed/versioned subtitle result and preserves the original; it cannot guarantee alignment. Anime ASS/SSA, multiple editions/cuts and archive handling require later explicit parser contracts. Respect provider quotas; never rotate endpoints/accounts to evade limits.

## Phase 7: opt-in automation

Monitoring, automatic acquisition, quality upgrades, list-triggered acquisition, subtitle upgrades and synchronization are separately enabled policies with visible history and pause controls. Retain torrent identity reconciliation, worker leases and file replacement confirmation. Enabling a source, creating a profile or opening a library never enables these automatically.

## What this does not change

One local modular application, Go/React/SQLite, local-only access and stored identifiers remain as agreed. The guide is a workflow reference rather than an authority to delete files, weaken authentication, expose remote access or reuse third-party credentials. Player/streaming-server replacement remains outside this manager's current scope.
