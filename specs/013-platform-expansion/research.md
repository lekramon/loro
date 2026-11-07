# Native platform expansion research

Checked 2026-11-06. Public-source research for future specifications, not implementation evidence. Radarr/Sonarr/Prowlarr/Bazarr are behavior references; Lôro should own these responsibilities in its Go modular monolith. Preserve the title/file/acquisition boundaries, local storage and recovery rules in [ADR-0001](../../docs/adr/0001-local-modular-application.md), [ADR-0002](../../docs/adr/0002-jobs-and-file-recovery.md) and the [product specification](../000-product/spec.md).

## Settings mapping

Radarr's [settings documentation](https://wiki.servarr.com/radarr/settings) and [settings source tree](https://github.com/Radarr/Radarr/tree/develop/frontend/src/Settings) establish these functions; the last column is a proposed Lôro seam. System is an operational area alongside Settings.

| Area | Radarr function | Suggested Lôro seam |
|---|---|---|
| Media Management | Naming, folders, import/hardlinks, file handling, permissions and roots. | Library/import policy: root validation, naming preview, copy/hardlink capability and recoverable publication. Preserve files and require explicit replacement. |
| Profiles | Allowed/ranked qualities, upgrade cutoffs and delay policies. | Versioned acquisition Profile and later monitoring policy. Keep viewer profiles separate; specify multiple named acquisition profiles separately. |
| Quality | Quality definitions and minimum/preferred/maximum MB/min. | Candidate constraints with explicit units and duration evidence. Unknown size/duration stays unknown; separate resolution, source format and container. |
| Custom Formats | Conditions identify release characteristics; profile scores affect selection. | Typed predicates over normalized evidence, mandatory exclusions, preference scores and explanations. Avoid copied JSON/regex languages. |
| Indexers | Source credentials/capabilities, search/RSS modes, filters and priorities. | Existing Source adapters, tests and provider-scoped limits. Separate manual search from later polling/monitoring. |
| Download Clients | Transfer connections, categories, completion/failure handling and path mappings. | Existing qBittorrent adapter, ownership and reconciliation; import remains a separate service. State precisely what connection tests verify. |
| Import Lists | Fetching, auto-add/search defaults, exclusions and optional cleanup. | Native adapters → snapshot → identity resolution → preview → idempotent catalog additions. No deletion from absence. |
| General | Host/security, proxy, logging, updates and backup configuration. | Private credentials, revision-safe settings and existing bootstrap/deployment ownership. |
| System | Health, storage, scheduled tasks, backups, updates and diagnostics. | Local readiness distinct from upstream health; durable job state, last success/failure, sanitized history and reviews. |

Radarr source separates [profile cutoff/upgrade flags and format scores](https://github.com/Radarr/Radarr/blob/develop/src/NzbDrone.Core/Profiles/Qualities/QualityProfile.cs) from [quality size definitions](https://github.com/Radarr/Radarr/blob/develop/src/NzbDrone.Core/Qualities/QualityDefinition.cs). Its [OpenAPI](https://github.com/Radarr/Radarr/blob/develop/src/Radarr.Api.V3/openapi.json) separately exposes configuration, profiles, qualities, custom formats, sources, clients, lists and health; use it as an inventory, not Lôro's wire contract. [System documentation](https://github.com/Servarr/Wiki/blob/master/radarr/system.md) distinguishes health, tasks/queue, backups, updates and events.

## Historical guide versus current behavior

The [requested Reddit guide](https://www.reddit.com/r/pirataria/comments/18ch7bt/guia_do_streaming_dom%C3%A9stico_automatizado_sonarr/) describes list membership → desired title → release preferences → download → organized library → Portuguese subtitles → external player. Retain that workflow intent. Its recommendations to accept all qualities, skip free-space checks, use a third-party Letterboxd bridge, delete movie files when removed from lists and synchronize subtitles automatically are personal historical choices, not Lôro defaults or current integration guarantees. Do not import its JSON, regexes or templates.

Current Radarr [FAQ](https://wiki.servarr.com/radarr/faq) distinguishes polling newly posted releases from an initial/backlog search; monitoring does not mean periodically searching every missing title. Model polling, search-on-add and upgrade decisions separately. Radarr's [torrent process](https://github.com/Servarr/Wiki/blob/master/radarr/settings.md#torrent-process) also conditions cleanup on client seeding/completion; Lôro's current preservation contract takes precedence over the guide.

## Native subtitles

[Bazarr setup](https://wiki.bazarr.media/Getting-Started/Setup-Guide/) connects to Radarr/Sonarr for metadata/file locations. Language profiles cover languages, forced/HI choices, audio exclusion and cutoffs, with defaults for new movies/series. [Subtitle settings](https://wiki.bazarr.media/Additional-Configuration/Settings/Subtitles/) distinguish embedded tracks, external sidecars, movie/episode match thresholds, upgrades, adaptive searches and optional ffsubsync processing. Synchronization can take minutes; there is no basis for importing the guide's near-universal success claim. [Providers](https://wiki.bazarr.media/Additional-Configuration/Settings/Providers/) differ in account, payment and CAPTCHA requirements.

Proposed ownership and behavior:

- Movie/series policy supports series inheritance and episode overrides. A series is a policy scope, not the owner of one shared subtitle file.
- Each subtitle asset belongs to a concrete library file identity/revision, with language/variant, format, provider ID, match evidence, hash, path and history. Episode/file coverage handles multi-episode files; title or path alone is insufficient identity.
- Movie/episode screens show status and manual search/download. Series actions enqueue bounded per-file work. Distinguish missing, embedded satisfaction, queued/searching, downloaded, no match, limited and needs review.
- Durable jobs snapshot file/policy/provider revisions. Revalidate file/root ownership before writing; validate and stage bounded subtitle text, then atomically publish a sidecar. Preserve existing/manual sidecars on conflicts and require an explicit replacement policy. Never modify hardlinked media contents.
- Keep Brazilian Portuguese distinct from Portuguese, forced/HI explicit, and unknown embedded languages unknown. Probe embedded tracks before declaring subtitles missing.

Native prerequisites: a maintained API adapter, application key where required, applicable user authentication, private write-only secrets, language/ID mapping, bounded search/download/parsing, provider cooldowns/quotas, sanitized errors and simulator fixtures. Start with one provider and manual movie sidecars after verified import, then monitoring and series inheritance.

OpenSubtitles documents an application API key and identifying User-Agent, search/download operations and optional user authentication. Its help pages give different free-account allowances: do not hardcode a marketing quota; handle returned limit/reset evidence. Resolve registration/distribution terms before bundling a key. Sources: [getting started](https://opensubtitles.tawk.help/article/getting-started), [API overview](https://opensubtitles.tawk.help/article/about-the-api), [application packages](https://opensubtitles.tawk.help/article/pro-packages). Native Go provider work does not require Bazarr's runtime. Full synchronization requires a separately specified/pinned processor or original implementation; ffprobe alone does not synchronize subtitles. [ffsubsync documentation](https://ffsubsync.readthedocs.io/en/latest/how_it_works.html).

## Letterboxd / IMDb lists

| Surface | Verified support and limitation | Lôro preparation |
|---|---|---|
| Letterboxd API | [Access](https://letterboxd.com/api-beta/access/) requires approval and currently excludes private/personal projects; [API docs](https://api-docs.letterboxd.com/) describe OAuth and paginated lists. | Do not promise API sync before granted access/credentials. Keep an adapter seam. |
| Letterboxd exports/RSS | [Export](https://letterboxd.com/user/exportdata/) provides ZIP/CSV including lists. The access page describes member RSS for diary entries/reviews/lists, not a guaranteed complete arbitrary-list mirror. | Start with user-provided list CSV; separately prove any RSS completeness contract. |
| Guide's Letterboxd bridge | The guide changes the hostname to a third-party service; its [connector repository](https://github.com/screeny05/letterboxd-list-radarr) is separate from official API access. | Do not bundle it or infer reliability from historical instructions. |
| IMDb exports | The July 2026 [Lists FAQ](https://help.imdb.com/article/imdb/track-movies-tv/faq-for-the-list-feature/GNQMN47VZSE7KW38) documents export and distinguishes title lists from people/image/video lists. | Bounded user-exported title CSV; preview/reject unsupported kinds. No IMDb password collection. |
| Radarr IMDb | [Settings](https://github.com/Radarr/Radarr/blob/develop/src/NzbDrone.Core/ImportLists/RadarrList2/IMDb/IMDbListSettings.cs) accept top250/popular/ur… watchlists. The [generator](https://github.com/Radarr/Radarr/blob/develop/src/NzbDrone.Core/ImportLists/RadarrList2/IMDb/IMDbListRequestGenerator.cs) explicitly rejects ls… custom lists and calls Radarr metadata service; [implementation](https://github.com/Radarr/Radarr/blob/develop/src/NzbDrone.Core/ImportLists/RadarrList2/IMDb/IMDbListImport.cs) sets a 12-hour minimum refresh. | Do not claim arbitrary IMDb-list URL support or copy an external Radarr dependency. Verify future official remote interfaces independently. |

Proposed ingestion contract: parse a bounded snapshot; resolve stable IMDb/TMDB IDs via the metadata adapter; retain origin/membership history; deduplicate across lists/catalog by canonical content type + external identity; preview new/already present/ambiguous/unsupported rows. Title/year-only matches require review. Confirmation adds titles idempotently without searching, monitoring or downloading unless separately enabled. Refresh failure, empty/partial responses and membership removals never remove catalog records, files, downloads or annotations. Honor exclusions. Exclude deleted-content exports unless intentionally selected. CSV ingestion prepares lists; it is not continuous synchronization.

## Anime and sequencing

Keep structural types movie and series, with seasons/episodes/file coverage for series. Anime is classification and matching policy applicable to either, not a competing third structural type. Sonarr's [SeriesTypes](https://github.com/Sonarr/Sonarr/blob/develop/src/NzbDrone.Core/Tv/SeriesTypes.cs) models Anime alongside Standard/Daily; this informs numbering/search policy, not a universal ontology. Record classification, alternate titles and absolute/season episode mappings without changing stable identity. Conflicting orders or uncertain pack coverage require review. Subtitle ownership follows actual files: anime movies remain movies; anime episodes remain episodes.

No live accounts, list endpoints or subtitle providers were tested. Upstream develop/wiki branches can change: pin protocol fixtures during implementation. Finish/verify the current native-source ticket before integration, then specify verified import, manual subtitle sidecars, CSV preview/addition, typed release rules/movie automation and series inheritance as bounded slices. Avoid a generic plugin engine and full parity in one ticket.
