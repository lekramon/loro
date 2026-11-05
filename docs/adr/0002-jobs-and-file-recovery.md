# ADR-0002: Durable jobs and recoverable imports

Status: accepted under the user's delegated decisions on 2026-11-04.

## Jobs

Persist job kind, payload version, acquisition ID when relevant, optional canonical destination, state, due time, attempt count, lease owner/token, lease expiry, and last sanitized error. States: queued, running, retry-wait, succeeded, failed, and needs-review. Four workers by default; allow one through eight. Claim due work transactionally; update completion only with the current lease token. Lease duration is 60 seconds with renewal every 15 seconds. Expired leases become eligible for reconciliation, not unconditional replay.

Acquire persistent resource locks transactionally for acquisition and import destination. Lease tokens fence database changes, but cannot fence filesystem operations alone. If renewal fails, cancel work and check ownership before publishing files. The OS-level single-instance lock prevents a second application from overlapping a paused process. Never expire and concurrently rerun an in-process filesystem operation that has not stopped; cancel it first and require its worker to exit. Startup after process death reconciles incomplete import journals before workers resume.

Metadata and search get at most three retries after the initial attempt, at 1, 2, and 4 seconds; respect a longer Retry-After within a 60-second ceiling, otherwise return a rate-limit error rather than retrying early. Use 15-second request deadlines. Non-transient errors are not retried. Poll downloads every 15 seconds without holding a worker between polls.

Before submission, persist torrent identity and acquisition intent. Parse magnet identity or hash retrieved torrent metadata. Deduplicate active acquisitions by title and candidate identity. If identity is unsupported or uncertain, stop for review instead of promising duplicate-free submission. Lost responses enter reconciliation: query qBittorrent by identity and match category/tags. Absence of evidence after timeout leaves an explicit uncertain state; manual retry requires another reconciliation first.

## Imports

Existing files are cataloged in place. New files use `Title (Year)/Title (Year) [Resolution].ext`; omit unknown resolution. Sanitize invalid characters, reserved Windows names, trailing spaces/dots, and path traversal. Use the persisted acquisition ID as the suffix for explicit keep-both imports.

Require client completion, an unambiguous file association, positive size, and an ffprobe video stream before automatic import. ffprobe uses an argument array, local-file protocols only, and a 30-second deadline. Uncertainty enters review; valid video does not prove title identity.

Resolve and validate configured roots and destinations, including symlinks/reparse points, immediately before effects. Hardlink only when supported on the same filesystem; fallback to copying through a unique temporary file in the destination directory. Preserve download paths for seeding. Never modify media contents through a hardlink.

Journal the source, expected size, destination, staging and backup paths, user confirmation, and phases before irreversible changes. Verify a copy's content checksum against the source; verify hardlink identity. Publish by rename within the destination filesystem. Reconcile on restart using recorded evidence, not filename alone.

Conflicts pause for skip, keep both, or explicitly confirmed replacement. Replacement stages and verifies the new file first, preserves the old path under a private backup name, then publishes the replacement and commits its library record. Delete the old library backup path only after success is recorded. A crash before commit restores or resumes from the journal; after commit, retry cleanup without rolling back a valid new import. Cleanup failures remain visible and retryable. Never delete the original download as part of replacement.

Ignored imports remain in history and can reopen when the downloaded file exists. Failed downloads are reported for manual retry or another selection; there is no automatic alternative acquisition in the first milestone.

## Existing-file review implementation

Existing-file association is a bounded synchronous local operation, distinct from downloaded-file import. It does not require a download-client completion record. Its ffprobe step has an eight-second budget inside the HTTP deadline and reads only an opened local handle with pipe-only protocols. Identity confirmation, filesystem checks and probing precede the short revision-checked SQL commit. Scan/review operations exclude each other in-process. A lost response is reconciled by reading inventory; no physical mutation journal is needed for this metadata-only step.
