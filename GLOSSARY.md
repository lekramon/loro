# Domain glossary

| Term | Meaning |
|---|---|
| Media title | A movie or series with stable internal identity and external metadata identifiers; distinct from its display name. |
| Catalog | Persisted metadata and the titles the user has registered. |
| Library | Media files available on disk and their associations to catalog titles. |
| Source | A configured indexer endpoint exposing acquisition candidates; initially Torznab. |
| Candidate | A normalized search result with its origin and known or unknown attributes. |
| Profile | The single editable default set of accepted resolutions, preferred resolution, accepted release formats/containers, and ranked audio choices. |
| Acquisition | The user's selection of a candidate for a title, including any confirmed profile override. |
| Download task | Tracking of an external client's transfer, linked to an acquisition and torrent identity. |
| Job | A durable unit of background work with scheduling, attempts, and a renewable execution lease. |
| Reconciliation | Checking observed external or filesystem state before repeating an uncertain operation. |
| Import | Validation and recoverable creation of a library entry from completed downloaded files. |
| Review | A pending user decision for ambiguous identification, destination conflict, or failed work. |
| Replacement | An explicitly confirmed import that removes the previous library path only after the incoming import is verified and committed. |
| Original audio | Audio matching the movie's metadata language; an unknown candidate language cannot satisfy it as a confirmed match. |
| Data directory | Local, private application storage for the database and application-managed caches; separate from downloads and media. |
| Readiness | Whether storage and startup initialization permit handling API requests; independent of onboarding and external service health. |

Release format describes a source or modifier such as WEB-DL or Remux. A file container is the file structure, such as MKV or MP4; it is independent of resolution and release format.

A **viewer profile** is a person or viewing persona shown by the mock media interface. It is distinct from the acquisition **Profile** above. Prototype viewer names, favorites and watched markers are browser-local demonstration state, not authentication.


## Expanded media and policy vocabulary

| Term | Meaning |
|---|---|
| Acquisition profile | A named policy for accepted and preferred releases, assigned to a media title; distinct from a viewer profile and a subtitle profile. |
| Quality definition | A quality class and its acceptable file-size expectations. |
| Custom format | A release trait evaluated from available evidence and scored by an acquisition profile. |
| Series | A title containing seasons and episodes. |
| Episode | A separately identified installment of a series, distinct from any file that contains it. |
| Anime classification | A classification of a movie or series used for organization; it does not replace that title's underlying identity. |
| Import list | A source of proposed catalog additions with provenance, distinct from a release indexer. |
| Subtitle profile | Preferred subtitle languages, accessibility flags and matching requirements. |
| Subtitle asset | A subtitle track or sidecar associated with a specific media-file version. |
| Communication event | A sanitized record of an application interaction or integration attempt, distinct from acquisition history. |
