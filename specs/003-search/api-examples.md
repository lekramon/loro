# Source and release search API handover

Open Settings → Release source to enter the endpoint and API key using a password field. Keep the key out of source files, command history and screenshots. Endpoint must be HTTP(S), without userinfo/query/fragment, and reachable from the service. The endpoint path is used exactly as entered.

Public read/test commands after saving configuration:

```powershell
$base = 'http://localhost:18084/api/v1'
Invoke-RestMethod "$base/source"
Invoke-RestMethod "$base/source/test"
```

PUT source is defined by [OpenAPI](openapi.json): revision=0 initially, then use the returned revision for edits. Name/url are required; blank/omitted apiKey preserves a saved key, clearApiKey=true explicitly removes it. Changing endpoint requires a new nonblank key or explicit clearing. A changed revision returns 409; reload before retrying.

GET source/test performs a fresh Torznab caps call, bounded retries/timeouts and no redirects. The result contains capabilities and checkedAt, never a key or provider diagnostics. Caps success does not prove search authentication. Saved last-success data survives failed subsequent tests and restart; source saves clear old capability evidence. Manual release search is not implemented in ticket 01.

## Search releases for a registered movie

```powershell
$catalog = Invoke-RestMethod "$base/catalog/movies?limit=25&offset=0"
$movieId = $catalog.items[0].id
$page = Invoke-RestMethod "$base/releases?movieId=$movieId&limit=25&offset=0"
$page.items | Select-Object title, resolution, format, container, audioLanguages, status
if ($null -ne $page.nextOffset) {
    Invoke-RestMethod "$base/releases?movieId=$movieId&limit=25&offset=$($page.nextOffset)"
}
```

The catalog must contain a movie and the source must have a successful saved capability test. Optional category must be a discovered ID. Read `nextOffset` rather than deriving it from the deduplicated item count. Response total can be null. IDs omit provider GUIDs, URLs and keys; they are not persisted acquisition IDs. No torrent retrieval or submission occurs.
