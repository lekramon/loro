# Offline backup and restore

Label: needs-triage
State: open
Blocked by: specs/005-import/issues/01-verified-import.md

Implement OPS-001 through OPS-006 from [the feature specification](../spec.md). Publish the CLI help/exit-code contract, archive manifest format, additive migration/rollback policy and sanitized operations status before implementation.

Acceptance requires stopped-service lock tests, private credential handling, archive traversal/size/symlink rejection, empty-destination restore, checksum verification, failure cleanup, restart persistence, Windows ACL/path checks and non-root Linux Compose checks. Never overwrite an existing data directory or include media/download payloads in the archive.
