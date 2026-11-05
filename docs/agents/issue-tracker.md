# Issue tracker: local Markdown

The user authorized local tracking and adapting the plugin convention to the existing `specs/` layout. This overrides the plugin's default `.scratch/` location. No remote tracker or repository is required.

## Layout

- One feature per directory: `specs/<NNN-feature>/`.
- The authoritative feature specification is `spec.md` in that directory.
- Tickets are individual files under `issues/<NN>-<slug>.md`, numbered in dependency order.
- Design, API contracts, and verification evidence live beside the specification.
- The product specification is an umbrella document, not one executable ticket.

## Operations used by skills

- Publish a spec: create or update the feature's `spec.md`; publish tickets as separate issue files.
- Fetch a ticket: read its complete file and the feature specification, including appended comments.
- Label: edit its `Label:` line using the triage vocabulary.
- Dependencies: `Blocked by:` contains repo-relative ticket paths, comma-separated, or `none`. A ready ticket may still be blocked by unfinished prerequisites.
- Claim: select the lowest-numbered unblocked ticket, set `State: in-progress`, and record the claimant under `## Comments`.
- Complete: set `State: done` only after acceptance evidence is recorded under `## Verification`; include commands, results, and limitations. Commit the ticket on its own `feature/<ticket>-<description>` branch, push that branch, integrate into `main`, and push `main` before starting the next ticket. No PR is required for this user-authorized direct integration workflow.
- Failed or incomplete verification leaves the ticket open or in progress. `ready-for-agent` describes specification readiness, not implementation success.
- Comments: append under `## Comments`; preserve earlier evidence.

If an implementation skill uses a task graph, these file references are its blocking edges. Keep `specs/README.md` as the feature-level map; detailed tickets are authoritative for execution state.
