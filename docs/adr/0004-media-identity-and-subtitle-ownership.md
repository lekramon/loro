# ADR-0004: Media identity and subtitle ownership

Status: accepted planning default under the user's 2026-11-06 platform expansion.

## Decision

Keep movie and series identity distinct. Anime is a classification of either, rather than a third incompatible identity namespace. Episodes have identities separate from files; an ambiguous or multi-episode association requires explicit mapping. Preserve existing movie IDs/APIs when series are introduced.

Subtitle assets belong to a concrete media-file version, not merely to a title or series. Series pages may summarize subtitles, but each sidecar/embedded track is associated with its movie or episode file and matching evidence. Replacement invalidates that evidence without silently deleting user sidecars.

Use acquisition profile, viewer profile and subtitle profile as separate concepts. Import lists propose catalog additions and do not own the local media lifecycle; remote removal cannot authorize local deletion.

## Reason and consequences

A separate anime identity would duplicate movies/episodes, split acquisition history and complicate later classification changes. File-independent subtitles would appear valid after a different cut/edition replaces the video. Explicit ownership avoids both failures at the cost of more careful association/migration work. Concrete schemas and recovery journals remain ticket prerequisites; these domain decisions do not claim implementation. See the [platform specification](../../specs/013-platform-expansion/spec.md).
