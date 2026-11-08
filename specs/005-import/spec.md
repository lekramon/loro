# Verified media imports

Label: ready-for-agent
State: specified

This feature owns explicit import of completed downloads into the local library. It does not start downloads, delete download payloads, or infer title identity from a filename alone.

## Contract

- IMP-001: `GET /api/v1/imports` returns durable import history with bounded pagination and states `preview`, `awaiting-review`, `staging`, `published`, `failed`, `ignored` and `needs-review`.
- IMP-002: `POST /api/v1/imports/preview` accepts exactly `{downloadId, revision}`. It requires a completed owned download, rechecks the download revision and settings roots, enumerates bounded files, runs ffprobe with local-file protocols and returns candidates without writing media or changing history.
- IMP-003: `POST /api/v1/imports` accepts exactly `{downloadId, fileId, movieId, revision, conflictPolicy}`. It persists the source, destination, policy and journal before staging. `movieId` is explicit; ambiguous or missing association enters review.
- IMP-004: Destination names are `Title (Year)/Title (Year) [Resolution].ext`, with unknown resolution omitted. Sanitize invalid characters, reserved Windows names, trailing spaces/dots and traversal. Keep-both appends the import ID.
- IMP-005: Require positive size, completed ownership, unambiguous association and an ffprobe video stream. Unknown or unavailable evidence never becomes an automatic import.
- IMP-006: Prefer a same-filesystem hardlink when supported; otherwise copy to a unique destination temporary file, verify size and checksum, and publish by same-filesystem rename. Never modify the download through a hardlink.
- IMP-007: Conflicts require `skip`, `keep-both` or explicit `replace` confirmation. Replacement verifies the new file first, keeps the old library path under a private backup name, publishes the new path, commits the library record, then cleans the old backup. Cleanup is retryable and never deletes the original download.
- IMP-008: Journal phases are `planned`, `staged`, `verified`, `old-preserved`, `published`, `committed` and `cleanup-pending`. Startup reconciles every incomplete journal before workers resume; it uses recorded evidence, not filenames alone.

The migration is additive and must preserve catalog, inventory, downloads, acquisitions and user files. Rollback is a stopped-service backup restore; a destructive down migration is not supported.
