# Verification plan

Phase zero uses the existing native-source branch/ticket. Add an outbound HTTP simulator that receives secret canaries in every private field and returns secret-bearing bodies/errors; inspect diagnostics through the public API and private rotating files and assert absence of those canaries. Verify response status/latency, separate retries, cancellation and operation correlations. Do not depend on a public tracker's availability for these assertions.

Exercise a real temporary diagnostics directory across restart; verify bounded retention/rotation, truncated records, permission/write failures, session cursor resets and stale cursor truncation. Confirm that diagnostics polling does not grow the log. Confirm a failed logger does not change source, download or job outcomes.

For the browser, use local fixture pages for a slow streaming result, a challenge that clears, a challenge that persists, malformed parsing, process startup failure and cancellation. Assert distinct phase evidence and that a widget click alone cannot produce success. Opt-in live validation records the exact final code and counts, without cookies, query strings, titles or page bodies.

Browser UI coverage: open the bottom-left drawer from setup/settings/library, see a pending request before it completes, filter/pause/resume, reopen without duplicate polling, and inspect en-US/pt-BR/light/dark/mobile states. Later ticket 01 covers every settings route and honest Series/Anime planned views.

Before phase-zero completion, rerun the public API suites and Go tests/vet on Windows and non-root Linux; build the production frontend and image, lint all API contracts, validate Markdown dependencies, and run Compose persistence/file-preservation verification. Save results separately from intended behavior and keep incomplete work off main.
