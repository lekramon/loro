# Direct sources and local browser runtime

Settings → Release source → Source type offers Torznab and **1337x (direct)**. The direct adapter takes an origin such as `https://1337x.st/`, without an API path or key. Use the canonical site origin. Results that end on another origin are rejected; browser HTTP redirects may be contacted before that check. No saved source credentials are passed to the browser. Saving is explicit and replaces the single configured source; it does not change Prowlarr/Radarr. Test the saved source, then search a registered movie from Releases.

Lôro launches an ephemeral browser for the direct adapter. It manages the process, deadline and challenge handling itself; there is no Byparr/FlareSolverr service or solver URL. Go remains the backend; this optional integration uses a Python/browser child runtime. A site may still reject a browser or require unresolved verification. The UI distinguishes a missing runtime, unresolved challenge, timeout and malformed page.

## Windows development

Use Python 3.13 and run from the repository:

```powershell
python scripts/install_browser.py
```

The installer pins all Python dependencies in `browser-requirements.txt` and fetches the sealed browser with upstream checksum verification. Default installation is `%LOCALAPPDATA%\LouroBrowser`; browser caches remain local outside the checkout. No runtime is installed while serving ordinary requests. If using a custom installation, set `LOURO_BROWSER_PYTHON` to its absolute `Scripts\python.exe` before starting Lôro. Keep the installer and service under the same OS user/cache environment. Packaged desktop shells can redirect AppData; use the absolute path printed by the installer when launching outside that shell.

## Linux Compose

The runtime image includes Python 3.13 from the pinned Debian snapshot, the dependency lock, Firefox dependencies, upstream license notices and the sealed browser cache. It runs under the existing non-root user. No additional container is needed. The browser cache is reproducible and excluded from application backups.

## Behavior and verification

Search retrieves movie-category pages. The source currently returns up to 40 rows; the API exposes windows of 20 using raw row positions, so mixed categories do not cause offset drift. Audio and technical attributes without evidence remain unknown. Only explicit identity preparation visits a selected details page, validates one magnet and privately records the resolved reference for the existing submission workflow. Search and preparation do not start downloads.

Run `python internal/browser/test_helper.py` for parser fixtures. Full Go tests cover source preservation, missing runtime, cancellation and child-process cleanup. Opt into real local-browser tests with `LOURO_TEST_BROWSER=1`; run `TestInstalledBrowserWithLocalFixture` and `TestInstalledNativeSourceSearchAndPreparation`. These use disposable local HTTP fixtures rather than a public tracker. Real tracker results are separate evidence, because HTML, challenges and network availability change.

Read [source specification](../specs/003-search/spec.md) and [research](../specs/003-search/browser-research.md). Schema 11 is additive; return to an older release using a matching stopped-service backup, not a schema downgrade.

## Communication history

The bottom-left Logs control is available during setup, in Settings and in the library. Open it before testing a source; filter to browser for navigation, challenge, widget interaction, parser and cleanup phases. A browser launch or widget interaction does not prove clearance. Only a valid non-challenge result emits challenge cleared; unresolved verification remains source-challenge-required.

The viewer also shows application API and outgoing metadata, Torznab, selected torrent-reference and qBittorrent communications. Status, latency, correlation IDs, retry delays and classified errors are retained; URLs, credentials, cookies and raw bodies are omitted. Pause affects display only. Polling stops when the drawer closes.

History is bounded to 2,000 in-memory events and three private NDJSON files of at most 2 MiB under the data directory's logs folder, retained for seven days. Recent history survives restart. Failed log storage leaves memory recording available and displays a warning; it does not fail source or acquisition work. These are application diagnostics, not a capture of other programs' traffic.
