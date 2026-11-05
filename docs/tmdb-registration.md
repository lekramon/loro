# TMDB application credentials

## Decision

Use application-level API read credentials registered for **Piratarr**. This supersedes the original decision to require a token from each installation's user.

The Go metadata adapter will make direct HTTPS requests to TMDB API v3 with the application read token in the Authorization Bearer header. Users will not sign in to TMDB or enter a personal TMDB token during normal Piratarr setup. Metadata and images are cached locally; no Piratarr cloud proxy is introduced.

## Registration and provisioning

The maintainer registers the application through [TMDB account API settings](https://www.themoviedb.org/settings/api), using the Piratarr project name and an accurate description of the application. Complete the registration flow and obtain its application API Read Access Token.

Provide that credential through the maintainer's private release-build configuration described below. Do not add a real credential to tracked source, specifications, tests, command examples, or browser assets. Test with dummy credentials and HTTP simulators; live integration checks explicitly opt in to the registered credential.

Official release packaging will supply the credential to the backend so ordinary users do not have to configure it. A bundled credential can be recovered from a distributed application; build configuration keeps it out of source history, not secret from the machine running Piratarr. Do not use account passwords or TMDB user-session credentials for this purpose.

The [catalog feature specification](../specs/002-catalog/spec.md) defines the packaging hook, missing/rejected credential behavior and rotation procedure. Missing or invalid credentials disable new metadata queries with a clear diagnostic; existing cached catalog browsing and local setup remain available. Credentials never appear in API responses or logs. Source-only development does not silently fall back to credentials from another application.

## Attribution

When metadata integration ships, use an approved TMDB logo and the required attribution in About/Credits, following the provider documentation. Preserve attribution alongside metadata-derived screens as described in ADR-0001.

## Current state and verification

- The user supplied the Piratarr application read credential on 2026-11-04. Authentication against TMDB's configuration endpoint succeeded with HTTP 200.
- The development credential is stored in a user-only private file at `%LOCALAPPDATA%/Piratarr/credentials/tmdb-read-token`, outside the repository. No credential value is recorded here or in Git. Read the file directly when implementing an explicitly authorized live check; never print it.
- The catalog adapter now reads `--tmdb-token-file` (or `PIRATARR_TMDB_TOKEN_FILE`) for native/private-file provisioning. The path is bootstrap-only and is never returned through the API. Missing/unreadable/invalid files disable new authenticated metadata requests while readiness and persisted browsing remain available.
- Without a private file, the adapter uses an optional application credential embedded by release packaging. Ordinary source-only builds contain an empty placeholder and report metadata unconfigured. Users of a provisioned release are not asked for a TMDB token during setup.
- Runtime private-file rotation takes effect on the next provider call. Cached results do not trigger a provider call merely because the credential changed. Write the replacement file with the same private permissions.
- For release builds use a BuildKit `tmdb_token` secret and a new non-secret `TMDB_BUILD_REVISION` value. BuildKit secret contents do not invalidate cache automatically; change the revision on every provisioning/rotation. The backend compilation embeds the credential, then clears the temporary source placeholder. The final backend binary intentionally contains the registered application credential; the frontend never receives it.

```powershell
$registeredTokenFile = "$env:LOCALAPPDATA\Piratarr\credentials\tmdb-read-token"
docker build --build-arg TMDB_BUILD_REVISION=20261105-r1 --secret "id=tmdb_token,src=$registeredTokenFile" -t piratarr:release .
```

This command passes only a file path and public revision, never a token value. Keep the token file outside the repository. A local override may instead bind a private token file read-only into the container and set `PIRATARR_TMDB_TOKEN_FILE` to that container path; it takes precedence over any bundled credential.

The About page at `/about` serves the approved TMDB logo locally, with the exact provider notice. Metadata/catalog API responses link to `/api/v1/about` for client attribution.

An opt-in maintainer smoke check uses the registered credential against a disposable native catalog, then restarts with missing credentials and verifies durable movies and cached artwork:

```powershell
python "C:\Users\ramon\OneDrive\Documentos\ChatGPT\piratarr\scripts\verify_catalog_live.py" --token-file "$env:LOCALAPPDATA\Piratarr\credentials\tmdb-read-token"
```

Build `bin/piratarr-catalog.exe` first. The check never reads or modifies the existing configured installation and does not print credential values.

Sources: [application authentication](https://developer.themoviedb.org/docs/authentication-application), [registration and attribution FAQ](https://developer.themoviedb.org/docs/faq).


Environment note (2026-11-05): live catalog search/details/artwork verification passed with the registered private file before a host interruption. That file is currently unavailable at the documented location. Restore it through private maintainer provisioning before another live check; source-only/offline operation and simulator-based development continue without it.
