# 02 — Complete first-run setup through the browser

Label: ready-for-agent
State: open
Blocked by: specs/001-foundation/issues/01-start-local-application.md

## Outcome

A user explicitly chooses local directories, accepted/preferred resolutions, and ranked audio choices, then saves a single persistent profile through the web wizard.

## Context

Read [feature specification](../spec.md), [API contract](../openapi.json), and the root glossary. Use the foundation's settings tables and transaction boundary rather than introducing a second configuration store.

## Work

- Bundle ISO 639-1 codes with English names and expose them through setup state; include original audio as a distinct profile token.
- Implement path canonicalization and disposable write probes, rejecting overlapping roots and data-directory conflicts.
- Implement POST setup and complete GET settings, schema/service validation, atomic initial revision 1, and setup race protection. Two concurrent setup requests result in one success and one 409.
- Build the wizard with editable text paths, resolution set, preferred resolution, audio ordering, review, field feedback, and navigation to saved settings.
- Cover strict JSON decoding, duplicate keys, unknown fields, body bounds, content type, and required mutation header.

## Acceptance

Cover FND-005, FND-006, FND-007, FND-008 (setup repetition), FND-010 (setup transaction failure), FND-012 (mutation input), FND-013 (onboarding), and FND-015. Check symlink/reparse-point resolution where supported. Assert probe cleanup and preservation of user files. Restart with the same real temporary database and verify profile order and canonical paths.

## Verification

Pending implementation.

## Comments

No integration credentials are requested in foundation setup. No quality/audio values are silently selected for the user.
