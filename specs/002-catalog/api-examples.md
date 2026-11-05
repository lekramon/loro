# Catalog API handover

The catalog wire contract is [openapi.json](openapi.json). These operations are live backend behavior; `/preview` remains isolated mock data until the new frontend is integrated.

## Start an isolated native development instance

From a PowerShell terminal, build in the checkout and use an explicit, separate data root. Configure the service token in setup/settings. A private bootstrap file remains optional.

```powershell
Set-Location "C:\Users\ramon\OneDrive\Documentos\ChatGPT\piratarr"
go build -o bin/piratarr-catalog.exe ./cmd/piratarr
& .\bin\piratarr-catalog.exe --data-dir "$env:LOCALAPPDATA\PiratarrCatalogPreview\config" --listen 127.0.0.1:18084
```

Keep that terminal open. Complete onboarding at `http://localhost:18084/setup` before adding movies. Setup/settings offer a password field for the TMDB API Read Access Token with an explicit Save TMDB token button. Source-only builds without a private file or provisioned release credential expose offline catalog browsing and report new metadata queries as unconfigured.

In another PowerShell terminal:

```powershell
$catalogBase = 'http://localhost:18084/api/v1'
Invoke-RestMethod "$catalogBase/metadata/status"
Invoke-RestMethod "$catalogBase/metadata/search?query=Inception&language=pt-BR&page=1"
$registration = Invoke-RestMethod "$catalogBase/catalog/movies" -Method Post -ContentType 'application/json' -Headers @{'X-Piratarr-Request'='1'} -Body '{"tmdbId":27205,"language":"pt-BR"}'
$registration.movie
Invoke-RestMethod "$catalogBase/catalog/movies?limit=50&offset=0"
Invoke-RestMethod "$catalogBase/catalog/movies/$($registration.movie.id)"
```

The first add returns 201 and `created:true`; another add returns 200 and the same movie with `created:false`, even if provider credentials are subsequently missing. Registered metadata retains the language chosen on first registration. `hasMedia:false` means no verified library association exists; it does not establish whether a movie file is on disk.

## Frontend integration notes

- Search metadata with `query`, `language` (`en-US`/`pt-BR`), `page` (1–500) and optional `year`. Results preserve TMDB IDs and original-language information. Request page bounds are independent from provider total pages.
- Movie registration sends `tmdbId` and optional `language`; never send client-authored title/year/quality fields. Use returned internal `id` for catalog details.
- Browse with `limit` (1–100), `offset` and literal title filter `q`. Catalog reads do not contact the provider.
- `posterUrl` and `backdropUrl` are local API paths or empty strings. Render a placeholder when empty or unavailable. The browser must never contact TMDB directly or receive an Authorization token.
- Search responses expose `cache.hit`, `cache.stale` and optional `warning` code. Retain stale results visibly instead of presenting failure as an empty search.
- Use error `code` for UI messages. Authentication/unconfigured/unavailable errors are 503, provider timeout 504, rate-limit 429 with Retry-After, invalid provider data 502, missing title 404, invalid request 400/422 and missing setup 409. No upstream payloads or credentials are returned.
- Display the provider attribution on metadata-derived screens and link to `/about`. The attribution API `/api/v1/about` provides notice, provider URL and local logo URL.
- Automatic acquisition, file scanning, media association, profiles/accounts and playback are not implemented by this backend ticket.
