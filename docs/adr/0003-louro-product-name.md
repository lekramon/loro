# ADR-0003: Louro branding with persistent identity compatibility

Status: accepted under the user's Louro rename on 2026-11-05.

## Context

Piratarr identifiers already address private databases, instance locks, directories, credentials, viewer photos/browser preferences and Compose deployments. Renaming those together with display text could open a fresh installation, lose apparent access to existing data or bypass an old instance lock. The alternative requires a separate coordinated data/deployment migration.

## Decision

Use Louro as the product name and executable/image name. Retain persistent identifiers, repository/module paths, Compose project/service identity and the existing mutation header. Add preferred LOURO bootstrap environment names with PIRATARR fallback; keep both executable entry points on the same implementation and unchanged data defaults. New names do not imply a new data namespace. No automatic file moves or secret changes accompany branding.

This supersedes product-name references in ADR-0001/0002, while preserving their existing data paths and recovery contracts. Future acquisition should use the Louro product name when specifying new category/tag conventions; no such submission state exists yet.

## Consequences

Existing launch commands, credentials, browser state and file ownership remain valid. Some technical identifiers retain the historical name intentionally. A later persistent-identity migration needs its own preservation, concurrency and rollback contract. UI/About changes can roll back without a schema change. External credential registration remains recorded under its actual historical application name until changed at the provider.
