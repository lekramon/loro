# Browser handling and direct adapters

Research date: 2026-11-06. No private URLs or credentials are recorded here.

Byparr's [request handler](https://github.com/ThePhaseless/Byparr/blob/main/src/endpoints.py) navigates a browser page and returns content, cookies and user agent. Its [browser lifecycle](https://github.com/ThePhaseless/Byparr/blob/main/src/utils.py) uses InvisiblePlaywright; challenge processing is in a separate module. This requires a real browser, not only a longer Go HTTP timeout. Byparr is GPL-3.0; no implementation source is copied into Lôro.

Prowlarr's [FlareSolverr proxy](https://github.com/Prowlarr/Prowlarr/blob/develop/src/NzbDrone.Core/IndexerProxies/FlareSolverr/FlareSolverr.cs) detects a challenged upstream response and asks a compatible solver for browser state before retrying. Solving Lôro's request to Prowlarr's local API cannot clear a challenge on Prowlarr's separate upstream connection. The user chose direct adapters instead of exposing a replacement solver service.

[InvisiblePlaywright](https://github.com/feder-cr/invisible_playwright) 0.45.0 supports Python 3.11+ on Windows/Linux. Its wrapper has MIT/Apache-2.0 notices and the patched Firefox runtime is MPL-2.0. Keep distributed dependency notices intact. The upstream fetch command verifies its browser archive; installation is separate from serving requests.

Prowlarr's [1337x definition](https://github.com/Prowlarr/Indexers/blob/master/definitions/v11/1337x.yml) documents a site-specific HTML search/details flow. Lôro will implement its own bounded parser and selected-detail magnet resolution, retaining existing conservative profile/identity rules. This does not make other protected sites compatible automatically.

## Local diagnosis evidence

- Current selected source was Torrentio through Prowlarr. Its capabilities declared movie text/IMDb search, not TMDB-ID search.
- Exact Lôro request failed in 8.73 seconds because a two-second timeout replayed the query four times.
- The equivalent direct Prowlarr text query did not respond within 75 seconds. A quick empty response to an unsupported TMDB-ID query was not an equivalent test and is excluded as success evidence.
- The configured Byparr container was stopped. Prowlarr assigned the solver tag to 1337x/Kickass, not Torrentio. No Cloudflare challenge was established as the cause of the Torrentio timeout.
