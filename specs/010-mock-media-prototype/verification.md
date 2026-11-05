# Verification evidence

Verified on Windows on 2026-11-05.

- Production frontend build and TypeScript checks passed.
- All 13 Playwright journeys passed: eight existing foundation/interface journeys and five new prototype journeys.
- New coverage verifies profile selection/creation/editing, per-viewer favorites and watched-state isolation, persistence across reload, sorting/filtering/search, empty results, details, simulated player controls and invalid routes.
- Browser request capture verified no API calls and no external requests during the prototype media journey. Mock downloads pause/resume without real mutations; returning to real settings loads the separate existing application.
- Portuguese mobile light/dark layouts passed overflow checks, and Escape dismissed the navigation drawer. Blocked local storage and malformed saved profiles fall back to usable in-memory state.
- Reviewed dark profile selection, home, library, movie details and downloads screenshots, plus mobile home and light movie details. Reproducible screenshots live under ignored `test-results/`.
- `go test ./...` and `go vet ./...` passed, including API and persistence regression coverage.
- `python scripts/verify_specs.py` and `git diff --check` passed. API contract remains unchanged at eight operations.
- Rebuilt `bin/piratarr.exe` with the production frontend and all 24 local SVG illustrations.
- Started a separate native preview at `http://localhost:8081/preview`, using `%LOCALAPPDATA%/PiratarrMockPreview/config`; verified HTTP 200 for the page and a bundled poster. Background process ID at launch: 31656.

The catalog, profiles, metadata, downloads and player are mock UI state. Profiles/favorites/watched markers persist only in browser storage, not SQLite; simulated transfers/player state reset on reload. No authenticated accounts or actual playback/downloads are implemented. Linux container runtime validation remains separately pending while the Docker Linux daemon is stopped.
