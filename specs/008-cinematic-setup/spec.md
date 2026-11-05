# Cinematic setup and quality preferences

Supersedes earlier empty-resolution and text-only directory defaults.

## Acceptance

- Browse existing server directories in an accessible in-app folder explorer; retain absolute-path entry. The browser sees the server filesystem, including container mounts, rather than opening a client OS dialog.
- Explicitly create and use default sibling downloads/movies folders under the server home Videos/Piratarr directory, or /data in containers. Never create folders on GET. Application data cannot be selected.
- GET /api/v1/directories accepts path and offset; returns path, parent, directories and nextOffset. POST /api/v1/directories/defaults accepts {} and returns directories. `nextOffset` is -1 on the last page. Defaults return the selected directory pair with status 200; listing returns 200. Unsupported methods return 405; invalid/default-folder requests return 400/422, inaccessible listings return 422, private-data listings return 403 and stopping returns 503. Mutation body limits return 413, with 415 for non-JSON bodies. Both inherit loopback/origin controls; mutation requires the standard header and JSON. Errors reveal no filesystem internals.
- New setups accept every supported resolution, release format and container. User chooses a preferred resolution and may remove accepted choices. At least one of each is required. Remux is a release format, distinct from MKV/MP4 containers.
- Persist format/container exclusions transactionally. Existing profiles gain all formats/containers through migration; existing resolution/audio preferences remain unchanged. Omitted fields on older updates preserve saved choices.
- Visual direction superseded by [the Jellyfin-inspired interface](../009-jellyfin-interface/spec.md); responsive light variant, accessible sun/moon theme controls, English and Brazilian Portuguese. Do not imply unimplemented catalog functionality exists.

## Verification

Public API tests with disposable directories/database, persisted quality preferences and filesystem browsing/default creation; browser onboarding and theme/language journeys; production build, Go tests/vet, API lint and specification consistency. Linux container execution remains unavailable until Docker starts.
