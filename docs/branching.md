# Branching and release workflow

`develop` is the integration branch. `main` contains stable release points. Both branches preserve shared history and must never be force-pushed.

## Feature delivery

1. Update local references and create `feature/<ticket>-<description>` from `origin/develop`.
2. Keep one ticket and its specification/evidence changes on that branch.
3. Run the ticket's local checks, push the branch, and let GitHub Actions finish.
4. Integrate the passing feature into `develop`. Prefer a pull request when review or branch protection is available; a direct fast-forward is acceptable for sequential, authorized work.
5. Delete merged feature branches only as an explicit cleanup action. Their commits remain reachable from `develop`.

The feature branch must be rebased or merged with the latest `develop` before integration when the branch has diverged. Never rewrite a branch another contributor is using.

## Releases and urgent fixes

Promote a tested `develop` commit to `main` through a pull request or fast-forward release integration. Tag releases from `main`. Ordinary feature branches do not merge directly into `main`.

Create an urgent production fix from `main` as `hotfix/<description>`. After verification, integrate it into `main`, then merge or cherry-pick that exact fix into `develop` so the branches do not regress.

## Continuous integration

The CI workflow runs on `main`, `develop`, every `feature/**` push, pull requests targeting `main` or `develop`, and manual dispatch. Required jobs are:

- frontend build, OpenAPI/specification checks, Go module verification, tests, vet, and application build on Linux and Windows;
- Playwright browser journeys on Linux, with diagnostics retained only on failure;
- full Linux image build and disposable non-root Docker Compose acceptance verification.

The workflow has read-only repository permissions, cancels superseded runs on the same ref, uses pinned toolchain versions, and pins third-party actions to immutable commits.

Real TMDB, source, or qBittorrent credentials are never CI requirements. Integration simulators and disposable SQLite/directories cover those boundaries; opt-in live checks remain local evidence.

## Migration from the original history

`develop` was created at the existing `main` tip. Every published historical `feature/*` branch was already an ancestor of that commit, so the new integration branch contains all prior feature work without replaying merges or rewriting commits. Incomplete branches stay separate until their own acceptance gates pass.
