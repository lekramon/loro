# Verified import implementation

Label: needs-triage
State: open
Blocked by: none

Implement IMP-001 through IMP-008 from [the feature specification](../spec.md) and [the API contract](../openapi.json).

Acceptance requires real disposable SQLite/directories, ffprobe and download simulators, hardlink/copy verification, conflict/replacement recovery, restart journal reconciliation, Windows path checks and non-root Linux filesystem checks. Do not delete download payloads during import. Publish migration and rollback evidence before claiming completion.
