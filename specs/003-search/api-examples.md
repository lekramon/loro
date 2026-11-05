# Source configuration API handover

Open Settings → Release source to enter the endpoint and API key using a password field. Keep the key out of source files, command history and screenshots. Endpoint must be HTTP(S), without userinfo/query/fragment, and reachable from the service. The endpoint path is used exactly as entered.

Public read/test commands after saving configuration:

```powershell
$base = 'http://localhost:18084/api/v1'
Invoke-RestMethod "$base/source"
Invoke-RestMethod "$base/source/test"
```

PUT source is defined by [OpenAPI](openapi.json): revision=0 initially, then use the returned revision for edits. Name/url are required; blank/omitted apiKey preserves a saved key, clearApiKey=true explicitly removes it. Changing endpoint requires a new nonblank key or explicit clearing. A changed revision returns 409; reload before retrying.

GET source/test performs a fresh Torznab caps call, bounded retries/timeouts and no redirects. The result contains capabilities and checkedAt, never a key or provider diagnostics. Caps success does not prove search authentication. Saved last-success data survives failed subsequent tests and restart; source saves clear old capability evidence. Manual release search is not implemented in ticket 01.
