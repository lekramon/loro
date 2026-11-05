# Verification evidence

Verified on Windows on 2026-11-04.

- Production frontend build and TypeScript checks passed.
- All eight Playwright journeys passed: real server directory browsing, complete setup/settings editing with stale-tab conflicts, persisted format exclusions, language/theme switching and storage failures.
- Existing browser coverage now verifies compact page heading, selected workspace navigation and the current wizard stage.
- Reviewed desktop dark setup, desktop light folders/quality/saved settings, and mobile light screenshots. Mobile overflow check passed. Screenshot capture scrolls to the top to avoid sticky-navigation artifacts in full-page images.
- Specification consistency check passed: valid links and dependency graph, eight unchanged API operations.
- Native Windows executable rebuilt at `bin/piratarr.exe` with the updated embedded frontend.
- `git diff --check` passed.

No backend or schema behavior changed. Linux container runtime verification remains a separate pending packaging check.
