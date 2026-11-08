# Offline operations and recovery

Label: ready-for-agent
State: specified

Operations are local maintenance commands, not remote administration. They run outside the service API while holding the same data-directory lock.

## Contract

- OPS-001: `louro backup --output <archive>` requires a stopped service, acquires the exclusive data-directory lock, checkpoints and closes SQLite, and writes a private archive containing the database, private credentials and a manifest with application/schema versions. Exclude media, downloads and reconstructible caches.
- OPS-002: Backup archives reject absolute paths, traversal, symlink entries, duplicate paths, unsupported schemas and files exceeding bounded archive limits. Credentials remain treated as secrets; archive permissions are owner-only/restricted to the service account.
- OPS-003: `louro restore --input <archive> --data-dir <empty-directory>` requires an empty destination, validates the manifest and schema compatibility before extraction, stages outside the destination, atomically publishes only after verification, and removes partial restoration on failure. It never overwrites an existing data directory.
- OPS-004: Restore preserves catalog, settings, source/client credentials, acquisitions, download history and unresolved reviews. Media and downloads are not included and must remain untouched.
- OPS-005: Startup rejects newer schema histories, recovers incomplete import journals before workers resume and fails closed when recovery evidence is inconsistent. Schema downgrade is unsupported; restore a matching stopped-service backup to return to an older release.
- OPS-006: `louro backup --check <archive>` is read-only and validates the manifest, archive paths, checksums and schema without opening the application database or modifying the destination.

No HTTP endpoint changes application data for backup or restore. Readiness and communications may report the last successful maintenance result, but must not expose archive paths, credentials or private filesystem details.
