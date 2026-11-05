# Replace language dropdown with PT/EN toggle

Label: ready-for-agent
State: done
Blocked by: specs/007-interface-preferences/issues/01-theme-language.md

Implement UI-006 in the [interface specification](../spec.md), following the user's attached rounded PT/EN reference. Existing [foundation API](../../001-foundation/openapi.json) and other feature APIs remain unchanged; this is browser-local display state.

## Acceptance

- Shared rounded PT/EN control with selected, focus and accessible native-button states.
- English/Portuguese translation, persistence, defaults and form preservation keep working without settings mutations.
- Same header placement in library/settings, readable light/dark/mobile layouts and unchanged sun/moon behavior.
- Production build and existing browser journeys pass with button interactions; inspect screenshots against the reference.

## Verification

UI-006 verified. Production build/type checking and all 26 Chromium journeys pass against the fresh Windows executable. Existing locale journeys now use native PT/EN buttons; keyboard Space/Enter switches document language, active selection persists after reload, unfinished forms/audio preferences stay intact and no display change mutates settings. Shared Library/Settings geometry, blocked/invalid storage, light/dark and mobile checks pass. Cropped EN pill and full light mobile/dark desktop screens were visually compared with the supplied reference. No API/schema/storage-key/branding changes.

Linux packaging: `docker build -t piratarr:language .` passed, including the unchanged backend's full non-root Go tests/vet and production embedded UI. Local text/reference checks pass (56 Markdown files, 27 API operations, 21 tickets). No native Go SDK is installed; the pinned compiler produced the ignored Windows preview. Final `python scripts/verify_container.py --image piratarr:language --port 18082` passed non-root Compose, embedded UI, ffprobe, health/setup, path validation, exclusion, persistence and unchanged media/cleanup.

## Comments

Claimed by Codex on 2026-11-05 after publishing the qBittorrent connection ticket. Product-name suggestions are separate; no new name has been selected.
