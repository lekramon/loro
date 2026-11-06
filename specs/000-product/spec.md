# Lôro — Product Specification

Status: accepted product direction; published to the local Markdown tracker.
Label: ready-for-agent
State: specified

This specification synthesizes the conversation and approved development-preparation plan. It defines product behavior and the first usable milestone; implementation proceeds through separately specified vertical slices. Architecture defaults are accepted in the ADRs. The user confirmed the testing strategy. Conversation and specifications use English. Foundation startup, first-run setup, revision-safe settings editing and Linux Docker distribution are implemented. Movie metadata/registration, file inventory and explicit association, source configuration/search, download-client connection checks, durable manual selection and v1 torrent identity preparation are implemented. Download submission, persistent workers, completion tracking and imports remain subsequent slices.

The product name is **Lôro**, refined from Louro on 2026-11-05. Historical Piratarr registrations and persistent technical identifiers retain their compatibility meaning; see [rename contract](../012-louro-name/spec.md).

## Problem Statement

The user wants to manage movies and TV series through a single service, replacing Sonarr, Radarr, and Prowlarr modules with original implementations. Catalog, sources, searches, preferences, downloads, and organization should share configuration and history within one application.

All product storage must remain local: configuration, credentials, database, persisted metadata, images, history, and media files. Content discovery may query external services; the product must not require its own cloud backend.

## Solution

A self-hosted application with a web interface and original modules maintains the desired library and compares it with the existing library. It queries configured sources, presents or selects acquisition candidates, coordinates an external download client, and imports files with history and failure recovery.

The first usable milestone covers movies and manual selection: add a missing movie, search one source, choose a result, submit it to one download client, track completion, and import the file. Restarting the service must not lose the task or duplicate the acquisition.

Automatic monitoring, upgrades, TV series, and additional integrations follow in later milestones. The interface and model should support that evolution without requiring a separate product for each media type.

## User Stories

### Installation and storage

1. As an administrator, I want to run a personal installation on my equipment, so that I control its data and operation.
2. As an administrator, I want to configure data, download, and library directories, so that I can use my local storage.
3. As an administrator, I want diagnostics for inaccessible directories, so that I can fix configuration before write operations.
4. As a user, I want configuration and catalog data to survive restarts, so that I can continue without recreating records.
5. As a user, I want to browse the persisted catalog offline, so that my library remains accessible when integrations fail.
6. As an administrator, I want credentials stored locally and omitted from logs and public responses, so that integration access remains private.

### Catalog and existing library

7. As a user, I want to search metadata and add a movie, so that I can identify the title I want to acquire.
8. As a user, I want to distinguish titles with identical names or different release years, so that I avoid incorrect associations.
9. As a user, I want adding an existing title to avoid duplicates, so that the catalog remains consistent.
10. As a user, I want to scan an existing directory, so that I can discover files I already own.
11. As a user, I want to confirm or correct file-to-title associations, so that I can resolve ambiguous identification.
12. As a user, I want to distinguish a cataloged movie from an available media file, so that I know what is missing.
13. As a user, I want repeated scans to avoid duplicate file records, so that the inventory stays current.
14. As a user, I want a missing file to leave its catalog title intact, so that my desired library is preserved.

### Sources and search

15. As an administrator, I want to configure and test a source, so that I can verify connectivity and authentication.
16. As a user, I want to search candidates for a cataloged movie, so that I can compare acquisition options.
17. As a user, I want recognized and unknown attributes displayed distinctly, so that I do not mistake assumptions for facts.
18. As a user, I want normalized results with their origins preserved, so that I can compare options without losing traceability.
19. As a user, I want authentication failures, timeouts, and rate limits distinguished, so that I understand why a search failed.
20. As an administrator, I want bounded queries and controlled retries, so that requests respect source capacity.

### Downloads and imports — first milestone

21. As an administrator, I want to connect an external download client, so that file transfers are delegated to it.
22. As a user, I want to select a result manually and submit it to the client, so that I control the initial acquisition.
23. As a user, I want to track task status and errors, so that I know when intervention is needed.
24. As a user, I want repeated clicks to avoid creating the same acquisition multiple times, so that I avoid duplicates.
25. As a user, I want tracking to resume after an application restart, so that existing tasks are not lost.
26. As a user, I want a lost submission response reconciled before resubmission, so that a client-accepted download is not duplicated.
27. As a user, I want only completed, validated files imported, so that incomplete media is not marked available.
28. As a user, I want destination conflicts reported without silent overwrites, so that existing files are preserved.
29. As a user, I want insufficient space or permission denial to preserve the original, so that a failed import remains recoverable.
30. As a user, I want interrupted imports to be recoverable, so that database and disk state remain consistent.
31. As an administrator, I want to map client paths to application paths when needed, so that installations with different directory views can work.
32. As a user, I want files needed for seeding preserved according to import policy, so that the client is not disrupted unexpectedly.
33. As a user, I want acquisition history through import, so that I can understand each state change.

### Automation — after the first milestone

34. As a user, I want mandatory requirements and quality preferences, so that I can guide automatic selection.
35. As a user, I want candidate acceptance and rejection reasons, so that I can adjust my rules.
36. As a user, I want missing movies monitored, so that the service searches periodically for new options.
37. As a user, I want to pause monitoring, so that I can prevent new automatic submissions.
38. As a user, I want better versions acquired under an explicit policy, so that my library improves without accidental downgrades.
39. As a user, I want the previous version preserved until the new import is confirmed, so that I do not lose the only available copy.

### TV series, expansion, and operation — later milestones

40. As a user, I want to catalog series, seasons, and episodes, so that I can manage them in the same product.
41. As a user, I want to choose which episodes to monitor, so that I control my desired library.
42. As a user, I want multi-episode files and season packs represented, so that redundant acquisitions are avoided.
43. As a user, I want to review ambiguous episode numbering, so that incorrect episodes are not imported automatically.
44. As a user, I want partial imports to leave incomplete seasons visibly incomplete, so that availability reflects reality.
45. As a user, I want searches across multiple sources to continue when one fails, so that working integrations remain useful.
46. As an administrator, I want source and client health information, so that I can locate operational problems.
47. As an administrator, I want consistent database and configuration backups and restores, so that I can recover an installation.
48. As an administrator, I want to update an existing installation without losing data, so that I can adopt new versions.

### Interface preferences

49. As a user, I want light and dark themes saved in my browser, so that the interface suits my viewing environment.
50. As a user, I want Brazilian Portuguese and US English interface languages, so that I can use the application in my preferred language without changing audio preferences.

## Implementation Decisions

### Confirmed decisions

- Local product storage.
- Original modules replacing the named managers.
- Phased, specification-driven development.
- Tests primarily through the public API, using a real disposable database and temporary directories, with external integration simulators; a few web interface journeys.
- English for conversation and specifications.
- Go backend with standard net/http, React/TypeScript/Vite frontend, and SQLite.
- Linux x64 Docker Compose deployment, Windows development, localhost-only access.
- Pure Go SQLite driver, automatic startup migrations, and a bounded worker pool.

### Architecture

- A modular monolith with one web interface and persistent tasks.
- Separate catalog, library, sources, search, rules, downloads, import, scheduler, and history responsibilities.
- A shared pipeline for movies and TV series, with specific rules in the corresponding domain.
- A first milestone focused on movies and manual selection before automating decisions.
- An external download client behind an adapter; the core manages acquisition and tracking.
- database/sql with modernc.org/sqlite, explicit SQL, foreign keys, WAL, and a five-second busy timeout on connections; serialized writes.
- Embedded Goose SQL migrations before accepting traffic or starting workers; migration failures or unsupported newer schemas stop startup.
- Versioned JSON REST API under /api/v1; production frontend embedded in the Go executable.
- Four durable workers by default, configurable from one through eight, with renewable leases and per-acquisition and destination coordination.
- Three retries after the initial attempt for transient metadata/search failures; download polling every 15 seconds. Uncertain sends require reconciliation.
- TMDB API v3 with application-level read credentials registered for Piratarr, cached metadata/images, and required attribution. Release packaging supplies the application credential; normal users are not asked for a personal TMDB token. Authentication stays in the Go backend, with direct TMDB access and no Piratarr cloud backend.
- Native Torznab-compatible endpoint support, configured by the user, without another indexer manager dependency.
- qBittorrent 5.x WebUI API with a dedicated category, acquisition tags, and torrent-identity reconciliation.
- ffprobe video-stream validation before import.
- Detailed architecture and recovery contracts are authoritative in ADR-0001 and ADR-0002.

### Profile and onboarding behavior

- Interface preferences are separate from media profiles: light/dark and pt-BR/en-US apply immediately and persist per browser. Default to the browser's color preference and Portuguese when its primary language is Portuguese, otherwise English. They do not update the shared settings revision or reset an open form. See the interface-preferences feature specification.

- One editable default profile applies to all movies; multiple profiles and per-movie overrides are deferred.
- First-run setup requires a nonempty accepted resolution set and a preferred resolution within that set. Supported initial values are 480p, 576p, 720p, 1080p, and 2160p.
- Audio preferences are a nonempty ranked list of language choices and/or original audio. Unknown audio remains unknown; original audio is evaluated against the movie's metadata language when available.
- Onboarding accepts all supported resolutions, release formats (including Remux), and file containers initially. Users choose a preferred resolution and may exclude unwanted options. Audio ranking remains an explicit choice. All preferences remain editable.
- Folder selection includes an in-app server directory explorer, absolute-path entry, and an explicit option to create and use default folders.
- Manual search shows candidates with mismatch explanations and unknown attributes. Selecting a conflicting or unknown candidate requires explicit confirmation without changing the profile.

### File and review behavior

- Catalog existing files in place without renaming or moving them.
- Automatically import completed, validated, unambiguous downloads; request review for ambiguous associations or destination conflicts.
- New movie paths use Title (Year)/Title (Year) [Resolution].ext, omitting unknown resolution and sanitizing invalid filesystem names.
- Hardlink when supported on the same filesystem; otherwise copy through a temporary destination file. Preserve source downloads and seeding.
- Ambiguous-file review offers assignment to a catalog movie or ignore, without deleting the download. Reopen ignored reviews from history if the file still exists.
- Conflicts offer skip, keep both with an acquisition suffix, or replacement. Confirm each replacement while showing current/incoming files and explaining deletion after success.
- Preserve the previous library file until the new import is verified and committed, then delete the previous library path. Failed cleanup is visible and retryable.
- Failed downloads require a user retry or another manual selection; automatic alternative selection is deferred.
- Initial backup and restore are maintenance commands requiring the service stopped; include database/configuration and exclude media/caches. Restore only into an empty data directory.

### Model and behavioral contracts

- Media title, search result, acquisition, download task, and media file have distinct identities. Display names and paths are not title identities.
- A title may have multiple files. Series must support multiple episodes per file and multiple files per download.
- Repeated registration and scanning do not create duplicate equivalent records.
- Adapters isolate metadata, source, and client formats. Errors distinguish authentication, unavailability, timeout, and rate limits.
- Search preserves candidate origins. Missing attributes are not presented as confirmed facts.
- Tasks persist intent, external identifiers when known, state, attempts, and relevant errors. Restarts trigger reconciliation before repeating uncertain external effects.
- Imports record enough progress to recover interruptions between disk and database changes. They do not assume a single transaction spanning both.
- Write operations validate destinations against configured roots, including resolved paths and filesystem redirection.
- Transfer, conflict, and recovery policies follow the file behavior above and ADR-0002.
- History and diagnostics omit secrets. Retain operational history for 90 days while preserving unresolved reviews.

### Implementation boundaries and dependencies

- Concrete contracts for catalog, search, acquisitions, imports, and maintenance are specified per feature before implementation; foundation contracts are prepared now.
- Real metadata integration checks require the application credential issued for Piratarr, provisioned by the maintainer. The user supplied the development credential and live authentication was verified on 2026-11-04; its private local location is documented in the maintainer registration instructions. The metadata adapter, private-file hook and optional BuildKit release provisioning hook are implemented; source-only live queries require the private credential file to be available. Each installation still needs its own accessible Torznab endpoint/credentials, qBittorrent access, and ffprobe. Simulators remove these as development prerequisites.
- The first release is local-only with no account management. Container publishing is restricted to host loopback; runtime state lives outside synchronized source folders.
- Supported stable toolchains and application dependencies are pinned in manifests and lockfiles; see development documentation for build and verification commands.

## Testing Decisions

### Confirmed test boundaries

The primary verification boundary is the public application API. Tests exercise observable behavior with a real disposable database and real temporary directories. Controlled simulators replace external integrations. A few web interface tests verify critical journeys.

A useful test checks a product promise and survives internal restructuring. It does not check class names, private calls, or internal structures without a public contract. Recovery scenarios may interrupt operations at critical points, but final evidence must be observable in the application, simulated client, and files.

### Coverage by responsibility

- Catalog and library: identity, repeated registration and scanning, association correction, and offline browsing.
- Sources and search: normalization, origins, unknown attributes, limits, and classified errors.
- Downloads and scheduler: persistence, repetition, concurrency, and reconciliation after a lost submission response.
- Import: original preservation, conflicts, permissions, insufficient space, path boundaries, and interruption recovery.
- Rules, later: mandatory requirements, tie-breaking, comparison with existing versions, and decision reasons.
- TV series, later: episode coverage, multiple files, packs, and partial failures.
- Distribution: clean installation, migrations, and restoration into a new environment.
- Foundation: onboarding, profile validation, persistent settings, bootstrap failures, and localhost exposure; see the foundation feature contract.

The repository has no existing tests or testing conventions. Framework, harness, and simulation mechanisms will be chosen with the stack. Insufficient space and system errors may require controlled fault injection; tests must not exhaust the user's real disk.

### First milestone acceptance criteria

| ID | Scenario | Observable outcome |
|---|---|---|
| AC-001 | Add the same movie twice | Only one title is registered |
| AC-002 | Scan a library again | Known files are not duplicated |
| AC-003 | File identification is ambiguous | Review remains pending; the original is not changed automatically |
| AC-004 | Select a result and complete the download | The validated file is imported and the movie appears available |
| AC-005 | Repeat an acquisition request | A second equivalent download is not created |
| AC-006 | The client accepts submission but the response is lost | The application reconciles the task before attempting another submission |
| AC-007 | Restart during a download | Tracking resumes the existing external task |
| AC-008 | Restart between disk modification and import registration | Recovery completes or flags the operation without losing the original or reporting false availability |
| AC-009 | Destination lacks permission, space, or has a conflict | The failure is explicit; the original is not deleted and no silent overwrite occurs |
| AC-010 | Destination resolves outside allowed roots | The operation is rejected without writing outside the configured area |
| AC-011 | Metadata provider or source becomes unavailable | The persisted library remains browsable; dependent actions show recoverable failures |
| AC-012 | Inspect history or diagnostics | Acquisition is traceable without exposing credentials |
| AC-013 | Client and application paths differ | Configured mapping locates the completed file or reports an explicit error |
| AC-014 | Complete first-run setup | Explicit resolution/audio choices and valid directories persist across restart |
| AC-015 | Select a mismatching or unknown candidate | Submission requires explicit confirmation and leaves the profile unchanged |
| AC-016 | Review an ambiguous file | Assign or ignore preserves the download; ignored review can reopen if the file exists |
| AC-017 | Confirm a replacement | Previous library path is removed only after verified import and committed state; cleanup failure is visible |
| AC-018 | Import a supported file | ffprobe detects a video stream; invalid or ambiguous content is not automatically made available |
| AC-019 | Back up and restore stopped service | Database/configuration restore into an empty directory; existing data is never overwritten |

These criteria will be expanded into scenarios and tasks for each slice before implementation. The milestone is complete only when evidence exists; this document does not claim any criterion has passed.

## Out of Scope

- A product-owned cloud backend or dependency on product-managed remote storage.
- An original BitTorrent or Usenet engine.
- Media playback, transcoding, and a streaming server.
- SaaS, multiple users, and mobile applications in the first version.
- Initial compatibility with every indexer and client.
- Multiple named profiles, per-movie profile overrides, custom naming templates, and automatic reorganization of existing libraries in the first milestone.
- Executable third-party plugins as a Piratarr feature. Development-assistance plugins belong to the development environment.
- Automatic monitoring, upgrades, TV series, and multiple sources in the first manual milestone, although they belong to the later vision.
- Schedule or quantitative performance commitments without a defined stack, environment, and representative data volume.

## Further Notes

- The roadmap has four milestones: movies with manual selection, movie automation, TV series automation, and a distributable stable release.
- Later stories guide evolution; they do not make the first milestone depend on implementing the entire vision.
- Local Markdown tracking, standard triage labels, and single-context domain documentation are configured. The user approved adapting plugin conventions to the existing specs layout.
- This is an umbrella specification, not a single implementation ticket. Its ready label means product decisions are available to feature authors; only feature tickets authorize a bounded implementation slice.
- Foundation startup, first-run setup, and revision-safe settings editing are implemented with native/API/browser evidence recorded in their tickets. Linux Docker distribution is now verified; the first catalog backend ticket is implemented; next specify existing-file scanning and association. Product-level acquisition/import criteria are not yet satisfied.


## Mock interface exploration

The user authorized a populated Jellyfin-style prototype with viewer profiles and additional screens. See [mock media prototype](../010-mock-media-prototype/spec.md). This frontend-only slice is independently accessible at `/preview`; it does not claim that real catalog, playback, downloads or authenticated accounts are implemented. Viewer profiles in this slice are local demonstration identities and do not replace the saved acquisition quality profile. A production multi-user persistence/authentication design remains outside this mock implementation.


## Service credential configuration update — 2026-11-05

By user instruction, setup/settings now allow explicit saving or replacement of a TMDB Read Access Token. This supersedes earlier statements that onboarding must not offer token configuration. The service-local write-only token takes precedence over bootstrap/private-file and bundled credentials; those remain fallbacks. Empty input preserves the credential. See [service configuration](../011-service-configuration/spec.md).
