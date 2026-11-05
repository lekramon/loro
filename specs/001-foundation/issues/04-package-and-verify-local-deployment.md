# 04 — Run the foundation in Linux Docker Compose

Label: ready-for-agent
State: in-progress
Blocked by: specs/001-foundation/issues/03-edit-persistent-settings.md

## Outcome

A user can build from a clean checkout, prepare private local mounts, run the Linux x64 image, complete onboarding, and preserve settings across container recreation.

## Work

- Add a pinned multi-stage build with React assets embedded in the Go binary and a maintained Debian-based non-root runtime with CA certificates and ffprobe. Pin base image digests and package/tool versions used by the build, documenting updates.
- Add Compose with loopback-only published port, local `/config` mount, common `/data` mount, explicit container mode, restart policy, and healthcheck using a new `piratarr healthcheck` subcommand.
- Healthcheck issues a bounded request to the local readiness route and exits nonzero on failure. Document native and container setup, ownership, canonical paths, backup limitations, and build/test commands.
- Add ignore rules excluding runtime data, credentials, downloaded media, build outputs, and local environment files from Git and Docker context while retaining required build inputs.
- Run foundation acceptance checks and capture results in feature verification; report platform or daemon limitations explicitly.

## Acceptance

Cover FND-014 and FND-015, and recheck FND-005/FND-007 in Linux. Prove assets work without a Node.js runtime server, port publishing is loopback-only, ffprobe is installed, and container restart/recreation retains settings. Verify the service remains unprivileged and does not change permissions on existing media. Validate healthcheck before and after onboarding.

## Verification

Implementation prepared. Native healthcheck tests, full Go tests/vet, standalone Compose configuration validation and native readiness smoke check passed. Linux image build/runtime checks remain pending because Docker Desktop exits with an error before its engine responds. See [preparation evidence](../verification.md).

## Comments

Backup commands belong to the later operations feature. No real metadata/source/client credentials are needed for foundation release checks.

Claimed by Codex on 2026-11-05. UI design is proceeding independently in v0; this ticket changes distribution and healthcheck only.
