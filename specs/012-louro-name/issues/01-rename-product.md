# Rename product to Louro

Label: ready-for-agent
State: done
Blocked by: specs/007-interface-preferences/issues/02-language-toggle.md

Implement NAME-001–NAME-004 in the [specification](../spec.md), preserving the [catalog API](../../002-catalog/openapi.json) apart from About's display name. Record evidence separately from intended behavior.

## Verification

NAME-001–NAME-004 verified. See [evidence](../verification.md): production build/contracts/docs, full Linux Go tests/vet, Windows bootstrap/healthcheck and both entry-point builds, 26 browser journeys and disposable Compose all passed. Existing persistent identifiers were preserved.

## Comments

Claimed by Codex on 2026-11-05 after the user chose Louro. The user authorized the product rename; compatibility preserves existing installations rather than moving their data.
