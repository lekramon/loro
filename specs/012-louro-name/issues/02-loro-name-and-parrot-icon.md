# Apply Lôro spelling and supplied parrot icon

Label: ready-for-agent
State: done
Blocked by: specs/012-louro-name/issues/01-rename-product.md

Implement NAME-005–NAME-007 in the [identity specification](../spec.md). Read the [catalog API](../../002-catalog/openapi.json): only the descriptive About name changes; routes/fields remain compatible.

## Acceptance

- Name is Lôro in current product text, browser title and About API, with ASCII operational identity preserved.
- Exact supplied PNG appears as favicon, Library icon and About mark, loads locally and keeps its aspect ratio.
- Viewer photos, PT/EN control, themes, navigation and existing stored preferences remain working.
- Verify production frontend/contracts/docs, browser rendering and asset delivery, fresh native preview and Linux image packaging. Record source-asset checksum and evidence.

## Verification

NAME-005–NAME-007 pass. See [ticket 02 evidence](../verification.md): exact source/copy checksum, production build/contracts/docs, 26 browser journeys and non-root Linux image/Compose checks all pass. Accented title/API name and local favicon/navigation/About PNG are verified; reviewed both themes and mobile rail. Technical identifiers and existing viewer/data state remain compatible.

## Comments

Claimed by Codex on 2026-11-05 after the user supplied the parrot image and explicitly selected Lôro. The supplied file is copied as an asset; no image generation/edit is needed.
