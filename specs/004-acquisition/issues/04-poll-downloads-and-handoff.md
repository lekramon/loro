# Poll downloads and prepare import handoff

Label: needs-triage
State: open
Blocked by: specs/004-acquisition/issues/03-submit-and-reconcile.md

Specify durable download-task tracking by confirmed client/hash/category/tag identity, fifteen-second scheduled polling, bounded worker use, restart recovery, sanitized transfer/completion/failure history and ownership/configuration changes. Define unambiguous completed-file evidence and the handoff contract before import implementation. Do not assume a successful submission proves completion or file/movie identity. Preserve original downloads and existing client torrents.

## Verification

Pending concrete state, API and simulator contracts. Real integration checks require a configured client with matching paths; simulator development remains independent of live credentials.
