# Development

## Toolchain

Pinned versions are Go 1.27.3, Node.js 24.22.1, and pnpm 11.25.0. Dependency versions are recorded in `go.mod`, `go.sum`, `package.json`, and `pnpm-lock.yaml`. Place Go, Node.js, and pnpm on PATH. The local development setup downloaded a checksum-verified Go archive to `%LOCALAPPDATA%/PiratarrToolchains/go1.27.3/go`; add its `bin` directory to the current shell PATH if using that installation.

## Clean-checkout build

```powershell
pnpm install --frozen-lockfile
pnpm run check:api
pnpm run build
go test ./...
go build -o bin/piratarr.exe ./cmd/piratarr
```

The frontend build must precede Go build/tests because the backend embeds its generated assets. Build output is ignored by Git. On Linux, use `-o bin/piratarr` instead.

## Run

```powershell
go run ./cmd/piratarr
```

Open `http://127.0.0.1:8080` and follow **Start setup**. Supply separate existing absolute download/movie directories, choose accepted/preferred resolutions, and rank audio languages or original audio. Saving validates paths and persists one profile. Use **Edit settings** to change it; every save carries the loaded revision, and a stale tab must explicitly reload before editing again. Folder changes update configuration only and never relocate existing media. Catalog, search, and downloads belong to subsequent tickets and are unavailable.

Default native data roots are `%LOCALAPPDATA%/Piratarr` on Windows and `/var/lib/piratarr` on Linux. Prefer an explicit private writable directory on Linux when not running as a service. Never use the synchronized repository for runtime data.

Bootstrap flags override environment variables:

| Flag | Environment | Default |
|---|---|---|
| `--data-dir` | `PIRATARR_DATA_DIR` | Platform-specific private directory |
| `--listen` | `PIRATARR_LISTEN` | `127.0.0.1:8080` |
| `--workers` | `PIRATARR_WORKERS` | 4 (validated, workers not implemented yet) |

`PIRATARR_CONTAINER=true` permits non-loopback binding and changes the default data root to `/config`. Only enable it in an isolated container with a host-loopback-only published port. Container packaging is ticket 04.

For frontend development, keep the native backend running and run `pnpm dev` in another terminal. Open `http://127.0.0.1:5173`. The loopback Vite server proxies API requests to the backend at port 8080, rewriting browser Origin only in development. Production uses one same-origin endpoint.

## Checks

```powershell
pnpm run typecheck
pnpm run check:api
go vet ./...
go test ./...
python scripts/verify_specs.py
pnpm run test:browser
```

Go tests use temporary local directories, real SQLite, and child processes for instance exclusion and process-death recovery. They do not use real external credentials. Shut down with Ctrl+C. Startup errors are structured JSON on stderr and stop the process before an HTTP listener is opened.

Browser tests launch their own temporary-data backend on port 18080 and require Chromium installed with `pnpm exec playwright install chromium`. Keep that port available. Playwright stores screenshots and other results in ignored `test-results/`.

To cross-build for the first deployment target:

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -o bin/piratarr-linux-amd64 ./cmd/piratarr
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

Cross-compilation does not substitute for Linux runtime and filesystem tests. Run those when the Docker Linux daemon is available; the initial development environment has Docker installed but its daemon stopped.
