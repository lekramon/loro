# Jellyfin-style mock media prototype

The user requests a faithful Jellyfin-style interface, viewer profiles and populated mock screens before real catalog implementation. This explicitly authorizes simulated media and profiles, superseding the earlier restriction against mock catalog screens.

## Acceptance

- A dedicated `/preview` frontend route runs without setup, external services or credentials. Every preview screen visibly identifies itself as mock data. Real setup/settings remain separate and accessible.
- Match the official Jellyfin web visual structure: dark neutral surfaces, cyan accents, compact top bar with centered Home/Favorites tabs, collapsible navigation drawer, library tiles, landscape continue-watching cards, poster rows and title/year captions. Keep Piratarr branding and original local illustration assets.
- Provide populated profile selection, home, movie library, search, favorites, movie details, downloads and viewer profile editing screens. Movie lists support sorting/filtering; details support per-viewer favorite/watched toggles. Downloads simulate pause/resume and show progress without contacting a client. No playback or actual download is claimed.
- Viewer profiles are browser-local mock identities, not authentication or the global acquisition quality profile. Support profile creation, switching and name editing. Favorites, watched state and display names persist per viewer when local storage works; remain usable in memory when storage is blocked.
- Mock actions never mutate real configuration, download files or send external requests. All artwork ships locally; preview works offline. Routes use hash navigation under `/preview`, with browser back and reload support.
- Retain light/dark controls and English/Brazilian Portuguese. Keyboard navigation and small-screen layouts remain usable with no horizontal page overflow. Provide empty search/favorites states and fallback for invalid movie routes.
- Add preview links to the existing setup/settings shell. Existing API contract is unchanged; the server serves the embedded entry point for `/preview` only, and unknown API routes remain JSON 404.

## Verification

Public browser journeys cover navigation, profiles and isolated persistence, mock actions, sorting/filtering/search, reload and real-settings separation. Verify production build, backend regression tests, specification consistency, representative desktop/mobile screenshots and native preview rebuild.

## Reference

Official [Jellyfin screenshots](https://jellyfin.org/) were inspected as visual references. This implementation uses original React/CSS and local illustration assets; it does not incorporate upstream source code, logos or screenshot imagery.

## Updated viewer and shell behavior

[Unified library requirements](../011-service-configuration/spec.md) supersede the original multiple demo seeds and duplicate drawer/header: one initial viewer, explicit additional profiles, local photo editing, shared application header/sidebar and library-contained live metadata search. Viewer state remains browser-local and mock library records remain separate from real metadata.
