# Interface themes and languages

Label: ready-for-agent
State: implemented

## Problem and outcome

Users need a comfortable light or dark interface and complete UI copy in Brazilian Portuguese or US English. Interface language is independent of preferred movie audio and of the English development documentation.

## Behavior

- Expose Theme and Language selectors on all application screens, including onboarding, settings editing, loading, and errors.
- UI-006: Replace the language dropdown with the user's rounded PT/EN segmented control. PT selects pt-BR; EN selects en-US. Use equal segments in a compact pill, an inset selected segment, muted inactive text, and clear contrast in each theme. Both segments are native buttons with full native-language accessible names/tooltips and aria-pressed, inside a localized Language group. Preserve visible focus, Tab/Enter/Space operation, existing localStorage keys, browser defaults, document language and unfinished form values. Use this shared control in every header with identical settings/library geometry. Theme sun/moon controls keep their existing behavior. This presentation change adds no API, schema, runtime data migration or branding decision.
- Offer light/dark and `pt-BR`/`en-US`. Apply changes immediately without navigation or losing an in-progress form.
- Store these display preferences in browser localStorage, scoped to the application origin. They are per-browser preferences, not part of the shared media profile; no backend API or database migration is needed.
- Without a saved preference, use the browser's dark-mode preference and select pt-BR when its primary language is Portuguese, otherwise en-US. Ignore invalid saved values. If browser storage is unavailable, keep the UI usable with in-memory preferences.
- Set the document language and native control color scheme. Translate headings, labels, actions, hints, validation feedback, conflicts, loading and failure states, dynamic ranking actions, and displayed language names. Preserve paths, identifiers, quality values and audio-language tokens.
- Keep existing English API responses stable; translate known feedback in the UI with a localized fallback for unexpected messages.
- Changing display preferences must not send a settings mutation or change audio preferences/revision.

## Acceptance

- UI-001: Both themes cover backgrounds, panels, text, controls, focus indicators and errors; controls remain usable on mobile.
- UI-002: Portuguese/English switches update current screens, document language and audio-language display names immediately.
- UI-003: Explicit choices survive reload and navigation; browser defaults and invalid/unavailable storage have safe fallbacks.
- UI-004: Switching locale/theme preserves unfinished setup/editor values and does not mutate the media profile.
- UI-005: Portuguese validation and stale-edit feedback are localized, while persisted audio tokens remain unchanged.

## Verification

Verified on 2026-11-04:

- UI-001: computed colors and native color scheme switch correctly. Dark pt-BR desktop and light pt-BR mobile screenshots were inspected; the mobile page has no horizontal overflow. Controls have explicit accessible labels.
- UI-002: translated shell, wizard/editor, headings, controls, language names, validation and stale-tab feedback switch immediately; the HTML language follows the selection. English setup/edit regression journeys remain green.
- UI-003: explicit selections survive reload/navigation; invalid saved values fall back to Portuguese/system dark mode in a Portuguese browser. With storage blocked, errors and both preference selectors still work in memory.
- UI-004: directory inputs, resolution selections, audio ranking and wizard step survive locale/theme changes. Browser tests observe no profile mutation from display changes; explicit saves retain the original audio tokens.
- UI-005: validation and conflict feedback render in Portuguese, and existing conflict feedback switches back to English without another save.
- `pnpm run build` passed with TypeScript checking; `pnpm run test:browser` passed all seven Chromium tests, including existing setup/edit journeys and four new interface scenarios. `go test ./...` and Windows/Linux x64 builds passed. No backend schema or wire contract changed.

The first localization test run exposed labels that included select option text. The controls were corrected to use separate explicit labels, and the complete suite then passed. Linux runtime/container verification remains a separate foundation packaging task; these UI changes were verified in Chromium on Windows.

## PT/EN control verification — 2026-11-05

UI-006: production frontend build passes and all 26 browser journeys pass with the native language buttons. Space and Enter activate the focused segment; aria-pressed/document language and persisted selection agree. Existing tests prove form/audio preservation, browser defaults, blocked/invalid storage, no preference mutations, immediate localized feedback and equal library/settings header geometry. Reviewed the cropped EN control against the reference and light mobile/dark desktop headers; no horizontal overflow. New preview: bin/piratarr-language.exe, including the completed qBittorrent connection settings. Linux image packaging is recorded in ticket 02. No backend/API/schema behavior changed in this ticket.
