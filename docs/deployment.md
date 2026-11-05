# Linux Docker Compose deployment

This is the initial Linux amd64 distribution. Build/runtime verification status is recorded in [foundation evidence](../specs/001-foundation/verification.md). A stopped or failed Docker engine prevents container verification; native build success does not establish Linux compatibility.

## Prepare storage

Use local directories outside the source checkout and synchronized storage. The runtime defaults to UID/GID 10001 and never starts as root or changes existing media ownership/permissions. Override the numeric UID/GID to match an existing media owner if needed.

On Linux, prepare **new** installation directories (adapt the path and numeric owner):

```sh
sudo install -d -o 10001 -g 10001 -m 0700 /srv/piratarr/config
sudo install -d -o 10001 -g 10001 -m 0755 /srv/piratarr/data /srv/piratarr/data/downloads /srv/piratarr/data/media /srv/piratarr/data/media/movies
cp .env.example .env
```

Do not apply those ownership commands to an existing library without checking its owner first. For existing media, configure `PIRATARR_UID`/`PIRATARR_GID` to an appropriate non-root account and arrange write access. The configured account must own `/config` because startup enforces private directory/database permissions. Config and media roots must be separate; never place a database inside a media folder.

Edit `.env` with absolute `PIRATARR_CONFIG_DIR` and `PIRATARR_STORAGE_DIR` paths. Compose refuses missing mount source directories instead of creating root-owned folders. The common `/data` bind mount exposes downloads and movies on the same filesystem where the host permits it, retaining future hardlink feasibility. Cross-filesystem imports will use the later fallback policy.

## Build and run

```sh
docker compose config --quiet
docker compose build
docker compose up -d --wait
docker compose ps
```

Open `http://localhost:8080` for setup/settings or `/preview` for the mock media interface. Compose publishes only `127.0.0.1:8080`, with port 8080 inside the container. Arbitrary hostnames, remote access and host-port translation are outside the current Host validation contract.

During setup use container paths, for example `/data/downloads` and `/data/media/movies`. The optional default-folder button creates `/data/downloads` and `/data/movies`; both remain inside the shared mount. Existing files are cataloged in place in later work; changing folder settings never moves media.

Production React assets are embedded in the Go executable. The runtime includes CA certificates and ffprobe but no Node.js server or Go compiler. The image and Compose drop Linux capabilities and prohibit gaining new privileges. Container restart policy is `unless-stopped`; graceful stop allows the application's shutdown deadline.

```sh
docker compose exec piratarr piratarr healthcheck
docker compose restart piratarr
docker compose up -d --force-recreate --wait
docker compose down
```

Stopping/removing the container does not delete the bind-mounted configuration or media. Backups must include configuration/database together with the service stopped; implemented backup/restore commands belong to a later ticket. Do not downgrade a database with a previous application version; restore a matching backup instead.

## Native readiness command

```powershell
.\bin\piratarr-next.exe healthcheck --listen 127.0.0.1:8081
```

Use the name of the executable you built and the running service's port. The command accepts `--listen`, then `PIRATARR_LISTEN`, then `127.0.0.1:8080`. Wildcard container addresses map to loopback. It times out within three seconds, follows no redirects, ignores HTTP proxies, rejects non-local targets, and exits nonzero unless the readiness API returns its expected JSON. It does not acquire the instance lock or access SQLite. Readiness is independent of onboarding.

## Reproducible inputs and updates

The Dockerfile pins Go 1.27.3, Node 24.22.1 and Debian trixie image manifest digests, pnpm 11.25.0, and frontend/Go lockfiles. Runtime packages use the Debian and Debian-security snapshots dated 2026-11-04, with exact CA certificate and ffmpeg versions. Snapshot repositories pin transitive apt resolution; signed repository verification remains enabled. Snapshot expiry checks are disabled only for these immutable historical indexes.

Refresh base image digests, snapshot date and runtime package versions together when applying security updates, then rebuild and repeat verification. Pins are not an automatic security-update mechanism. Verify supported tags through the [official Go image](https://hub.docker.com/_/golang), [Debian package index](https://packages.debian.org/trixie/ffmpeg), and [Docker Compose service reference](https://docs.docker.com/reference/compose-file/services/).

The Docker context allowlist includes only build inputs. Runtime state, Git history, local environment files, credentials, media, node_modules and generated distributions are excluded. An optional context audit exports the actual Docker-selected input files to a new temporary directory:

```sh
docker build --target context-audit --output type=local,dest=/tmp/piratarr-context-audit .
```

Choose an empty output directory outside this checkout and inspect `/context` beneath it. Never add credentials to source directories merely to test this.

## Acceptance checks

With the image built and Linux Docker running, keep local port 8080 free and run:

```sh
python scripts/verify_container.py --image piratarr:local
```

This creates a unique temporary fixture and Compose project, prepares only those new directories using a short-lived root helper, and runs the service as 10001:10001. It verifies real HTTP setup, readiness before/after onboarding, inaccessible/overlapping/symlinked roots, second-instance exclusion, embedded assets, ffprobe, unchanged media content/mode, and settings across restart/recreation. It stops its own Compose project and removes only its verified temporary fixture. Existing user libraries are never mounted by this verifier.

Backend tests also run inside the Docker build as UID/GID 10001. Docker Desktop on Windows must have its Linux engine running; filesystem behavior on Windows-backed mounts may differ from native Linux. Keep Windows native development checks alongside Linux container checks.
