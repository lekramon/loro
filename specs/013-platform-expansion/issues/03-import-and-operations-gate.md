# Verified imports and operational recovery prerequisite

Label: ready-for-agent
State: done
Blocked by: specs/013-platform-expansion/issues/02-acquisition-policy.md

Implement Phase 3 in the [feature specification](../spec.md).

## Acceptance

Before claiming, publish executable tickets/contracts for existing roadmap features 005-import and 006-operations. Complete their file-preservation, restart and backup/restore evidence; this ticket records the gate and does not substitute a UI stub for imports.

Use public API tests with disposable SQLite/directories and integration simulators, focused browser journeys, Windows checks and non-root Linux filesystem/runtime checks. Persist product data outside the repository. Do not mark planned controls implemented. Follow the one-ticket feature-branch/main publishing workflow.

## Verification

The gate is complete as a specification milestone. Published executable contracts and implementation tickets:

- `specs/005-import/spec.md`, `specs/005-import/openapi.json` and `specs/005-import/issues/01-verified-import.md` define bounded preview, ffprobe validation, hardlink/copy publication, explicit conflict handling and restart journals.
- `specs/006-operations/spec.md`, `specs/006-operations/openapi.json` and `specs/006-operations/issues/01-backup-restore.md` define stopped-service backup/restore, archive safety, manifests, empty-destination restore and sanitized status.
- Existing ADR-0002 recovery rules remain authoritative for leases, import journals, replacement and startup reconciliation.

No import or backup implementation is claimed by this gate; the new feature tickets remain open for implementation.

## Comments

Prepared on 2026-11-06 under the user's phased expansion request. No additional product answers are required; needs-triage means wire/migration design remains to be made concrete before claiming.
