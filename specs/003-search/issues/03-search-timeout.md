# Bound slow searches without repeated timeout requests

Label: ready-for-agent
State: done
Blocked by: none

Implement REL-007 in the [specification](../spec.md) and [API contract](../openapi.json).

## Acceptance

- Allow slow release responses within a 65-second upstream budget and 70-second API budget; extend only this route's write deadline to 75 seconds.
- Keep capability discovery's existing short budget. Do not replay a timed-out release query. Preserve bounded retries for immediate transient failures and rate-limit cooldown.
- Distinguish an explicit Cloudflare challenge from invalid credentials without exposing upstream content.
- Cancel searches from the UI and on navigation; abort the upstream request, release exclusion, and allow another search.
- Verify slow success, cancellation, retry bounds, challenge classification, and browser recovery with disposable storage/simulators on Windows/Linux.

## Verification

- Red: `go test ./internal/httpapi -run TestReleaseSearchSlowSuccessWithoutRepeatedRequests -count=1` failed against the old transport: two calls for a valid 2.3-second response.
- Green: Linux non-root `go test ./...` and `go vet ./...`; focused source timeout replay test; native Windows API release/source suites passed.
- `pnpm run build`, `pnpm run check:api`, `python scripts/verify_specs.py` passed.
- Three `web/tests/releases.spec.ts` browser journeys passed, including cancellation and Portuguese challenge feedback. History is simulated independently to avoid unrelated fixture errors.
- Production Linux image built; `python scripts/verify_container.py --image louro:local --port 18082` passed. Initial Compose attempt raced the image rebuild; rerun after image publication passed.
- Live source remains unresponsive beyond 75 seconds when called directly. No live search-success claim, downloads, external configuration changes or media changes.

## Comments

Claimed by Codex on 2026-11-06. Live reproduction returned source-timeout in 8.73 seconds; a single equivalent direct Prowlarr text query also exceeded 75 seconds. This ticket removes premature cancellation and repeated expensive requests; it does not claim to repair an upstream outage or provide a browser solver. The user selected direct adapters for the subsequent native integration.
