# TMDB metadata and persistent movie registration

Label: ready-for-agent
State: done
Blocked by: specs/001-foundation/issues/04-package-and-verify-local-deployment.md

Implement ticket 01 in [the specification](../spec.md) and [API contract](../openapi.json), covering CAT-001 through CAT-009. Include credential packaging, local artwork, attribution and offline persistence. Do not implement scanning, acquisition, or a replacement for the v0 design.

## Comments

Claimed by Codex on 2026-11-05 after the user requested the next stage.

## Verification

Completed CAT-001 through CAT-009. See [verification evidence](../verification.md): Windows/Linux API and persistence tests, Linux race detector, 14 browser journeys, live smoke, dummy/source-only credential packaging and the complete Compose regression passed. The repeated live check is currently unavailable because the local private credential file is absent after the host interruption; it is recorded separately from the successful earlier live check.
