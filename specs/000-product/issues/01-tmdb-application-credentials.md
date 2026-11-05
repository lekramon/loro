# Use Piratarr-owned TMDB application credentials

Label: ready-for-agent
State: done
Blocked by: none

## Outcome

Replace the per-user TMDB token requirement with application credentials registered for Piratarr, without adding credentials or an unimplemented metadata adapter to the foundation.

## Acceptance

- Product specification and ADR-0001 specify application-level credentials and no personal-token prompt during normal onboarding.
- Maintainer registration/provisioning instructions distinguish the accepted design from registration or credentials actually obtained.
- Backend-only authentication, local caching, attribution, and direct TMDB access remain the selected architecture.
- Simulator-based development remains unblocked; real catalog queries require the registered application credential.
- No actual credentials or unrelated runtime changes are committed.

## Verification

2026-11-04: `python scripts/verify_specs.py` passed with 19 Markdown files and valid local references, API references, and foundation ticket dependencies. `git diff --check` passed. A targeted search confirmed the previous per-user token requirement was replaced in product/architecture documentation. No runtime files or actual credentials changed, so application tests were not repeated.

## Comments

2026-11-04: User explicitly selected credentials registered for Piratarr. Claimed on `feature/tmdb-application-credentials`. This ticket updates the metadata design only; it does not claim application registration or implement catalog queries.
