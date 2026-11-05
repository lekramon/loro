# TMDB application credentials

## Decision

Use application-level API read credentials registered for **Piratarr**. This supersedes the original decision to require a token from each installation's user.

The Go metadata adapter will make direct HTTPS requests to TMDB API v3 with the application read token in the Authorization Bearer header. Users will not sign in to TMDB or enter a personal TMDB token during normal Piratarr setup. Metadata and images are cached locally; no Piratarr cloud proxy is introduced.

## Registration and provisioning

The maintainer registers the application through [TMDB account API settings](https://www.themoviedb.org/settings/api), using the Piratarr project name and an accurate description of the application. Complete the registration flow and obtain its application API Read Access Token.

Provide that credential through the maintainer's private release-build configuration when the metadata feature is implemented. Do not add a real credential to tracked source, specifications, tests, command examples, or browser assets. Test with dummy credentials and HTTP simulators; live integration checks explicitly opt in to the registered credential.

Official release packaging will supply the credential to the backend so ordinary users do not have to configure it. A bundled credential can be recovered from a distributed application; build configuration keeps it out of source history, not secret from the machine running Piratarr. Do not use account passwords or TMDB user-session credentials for this purpose.

The catalog feature specification must define the concrete packaging hook, missing/rejected credential behavior, and maintainer rotation procedure before implementing live requests. Missing or invalid credentials disable new metadata queries with a clear diagnostic; existing cached catalog browsing and local setup remain available. Credentials never appear in API responses or logs. Source-only development does not silently fall back to credentials from another application.

## Attribution

When metadata integration ships, use an approved TMDB logo and the required attribution in About/Credits, following the provider documentation. Preserve attribution alongside metadata-derived screens as described in ADR-0001.

## Current state and verification

- Decision and product documentation updated; registration and real credential provisioning remain outstanding.
- No credential has been created, retrieved, embedded, or tested by this documentation change.
- The current foundation has no TMDB adapter or credential input, so no runtime or database change is required.
- Development and the next foundation ticket can continue without a TMDB token.

Sources: [application authentication](https://developer.themoviedb.org/docs/authentication-application), [registration and attribution FAQ](https://developer.themoviedb.org/docs/faq).
