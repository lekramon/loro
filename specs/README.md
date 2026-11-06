# Specification roadmap

Specifications and tickets are the local tracker. Foundation, movie metadata, file inventory and explicit association/review are implemented. Torznab source configuration, manual release search and profile explanations are implemented. Durable selection and v1 identity preparation are implemented; submission recovery/import remain subsequent slices.

| Feature | Outcome | Depends on | Planning state |
|---|---|---|---|
| 000-product | Product constraints and milestone boundaries | none | Accepted direction |
| 001-foundation | Healthy local application, onboarding, profile, persistence, embedded UI | none | Implemented and verified, including Linux Docker distribution |
| 002-catalog | TMDB movie registration, cached metadata, existing-file scan and manual association | 001 | Metadata/registration, local artwork/offline APIs, inventory and ffprobe association/review implemented; real Movies presentation remains mocked |
| 003-search | Native Torznab capabilities, manual search, normalized candidates and profile explanations | 002 | Source configuration, manual registered-movie search, normalization and profile explanations implemented and verified |
| 004-acquisition | qBittorrent submission, persistent workers, tracking and uncertain-send reconciliation | 003 | Connection checks, durable selection/override confirmation, v1 identity preparation and restart review implemented; submission/jobs/reconciliation remain the next contract |
| 005-import | ffprobe validation, hardlink/copy, review, replacement and restart recovery | 004 | Deferred specification |
| 006-operations | Stopped-service backup/restore, installation and upgrade verification | 005 | Deferred specification |
| 007-interface-preferences | Light/dark themes and pt-BR/en-US interface copy | 001 tickets 01–03 | Implemented, including shared rounded PT/EN control; display preferences remain separate from media profiles |

The first usable milestone requires 001–006. Subsequent milestones add automatic movie acquisition/upgrades, series, and broader integrations. Write each feature's concrete contracts and tickets before implementation; the table is not a substitute for those specifications. Implement the foundation first, then specify each next feature using the resulting code and contracts.

- [Cinematic setup and quality preferences](008-cinematic-setup/spec.md).

- [Jellyfin-inspired interface](009-jellyfin-interface/spec.md).

- [Jellyfin-style mock media prototype](010-mock-media-prototype/spec.md).

- [Service configuration and Jellyfin layout](011-service-configuration/spec.md): token configuration/testing, live movie search and persistent navigation implemented and verified.

Ticket 03 of service configuration adds a unified library/settings header, library-contained search, one initial viewer and local profile photo/pencil editing; implemented and verified.

- [Lôro product identity and compatibility](012-louro-name/spec.md): product branding, current executable/image and preferred LOURO environment variables preserve existing data identities.
