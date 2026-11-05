# Lôro

Use English for conversation, specifications, code, and project documentation.

Before implementation, read the feature specification, its API contract, and the ticket being implemented. Keep changes within that ticket's acceptance criteria. Update specifications when behavior changes; report verification evidence separately from intended behavior.

When changing branding, bootstrap settings or stored identifiers, read [name compatibility](docs/adr/0003-louro-product-name.md). Historical Piratarr storage/deployment identifiers intentionally remain compatible.

## Agent skills

### Issue tracker

Use local Markdown under `specs/`, with one file per ticket. Read [tracker operations](docs/agents/issue-tracker.md) before creating, claiming, or resolving work.

### Triage labels

Use the standard five labels and a separate execution state. See [triage labels](docs/agents/triage-labels.md).

### Domain docs

Single-context repository. Read [GLOSSARY.md](GLOSSARY.md) and relevant [ADRs](docs/adr/) before domain or architecture changes. See [domain conventions](docs/agents/domain.md).

## Engineering

- Implement each ticket on its own `feature/<ticket>-<description>` branch created from the latest `main`; the user's branch naming convention overrides the default `codex/` prefix.
- After the ticket's required checks pass, commit its code, specification updates, and evidence together, push its feature branch, integrate it into `main`, and push `main` before starting the next ticket. Use fast-forward integration for sequential work; never force-push shared history.
- Keep incomplete work off `main`. Record unavailable checks and apply the ticket's completion criteria before integration. Leave the checkout on `main` after publishing a completed step.

- Product requirements: [product specification](specs/000-product/spec.md).
- Foundation work: [foundation specification](specs/001-foundation/spec.md).
- Cross-cutting decisions: [architecture](docs/adr/0001-local-modular-application.md) and [recovery](docs/adr/0002-jobs-and-file-recovery.md).
- Verify behavior through the public API with real disposable SQLite databases and directories; simulate external APIs. Test filesystem behavior in Linux containers as well as Windows development checks.
- Keep runtime databases, credentials, and media outside the synchronized checkout. Use temporary roots for tests.
- Preserve user files on failure. File replacement requires explicit confirmation and successful import verification before removing the old library path.
- Pin stable toolchains and dependency versions when adding executable projects; commit dependency lockfiles. Report checks unavailable in the current environment rather than claiming they passed.
