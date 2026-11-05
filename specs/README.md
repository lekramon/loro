# Specification roadmap

Specifications and tickets are the local tracker. Foundation startup and onboarding are implemented; subsequent slices remain planned.

| Feature | Outcome | Depends on | Planning state |
|---|---|---|---|
| 000-product | Product constraints and milestone boundaries | none | Accepted direction |
| 001-foundation | Healthy local application, onboarding, profile, persistence, embedded UI | none | Startup and onboarding implemented; settings editing and packaging next |
| 002-catalog | TMDB movie registration, cached metadata, existing-file scan and manual association | 001 | Next specification |
| 003-search | Native Torznab capabilities, manual search, normalized candidates and profile explanations | 002 | Deferred specification |
| 004-acquisition | qBittorrent submission, persistent workers, tracking and uncertain-send reconciliation | 003 | Deferred specification |
| 005-import | ffprobe validation, hardlink/copy, review, replacement and restart recovery | 004 | Deferred specification |
| 006-operations | Stopped-service backup/restore, installation and upgrade verification | 005 | Deferred specification |

The first usable milestone requires 001–006. Subsequent milestones add automatic movie acquisition/upgrades, series, and broader integrations. Write each feature's concrete contracts and tickets before implementation; the table is not a substitute for those specifications. Implement the foundation first, then specify each next feature using the resulting code and contracts.
