# rfc6035-2otel

An RFC 6035 SIP voice-quality (`vq-rtcpxr`) collector: it listens for SIP PUBLISH, parses the report
body in both the standard dialect and the measured Poly pre-standard dialect, and exports
OpenTelemetry metrics and logs.

## Task interface

`just check` is the gate: `fmt-check`, `lint`, `vet`, `test`, `tidy-check`, `gen-check`, `build`,
`vuln`, `fuzz`. It is CI's `build-test` job verbatim. `just ci` is the sanctioned superset, adding `snapshot` (cross-compilation) and
`image` (a Docker daemon); CI runs those two in separate parallel jobs.

Run `just` with stdin from `/dev/null`. No recipe is marked `[confirm]`.

`dashboards/` and `alerts/` are generated from the builders under `grafana/` and are never
hand-edited, and so is the region between `<!-- BEGIN/END GENERATED SIGNAL CATALOG -->` in
`docs/signals.md`. `spec/signal-catalog.json` is the hand-maintained **input** those builders read:
edit the catalogue or the builder, then `just gen`. `just gen-check` fails on committed drift.

Off the Mac, when the remote gate runner is installed (`which gaterun`): `gaterun -- just ci` (or
`just check`), paced across lanes with `gaterun-slot <lane> -- gaterun -- just ci`; `.gaterunner.yaml`
is its contract. Run one test or package locally.

## Tracking work

Work is tracked in this repository's GitHub issues; they are the only queue. Do not add a checklist
file such as `ROADMAP.md` beside them.

- A pull request body says how it relates to an issue, on a line of its own: `Closes #N` with the
  `Acceptance evidence` table from `.github/pull_request_template.md` (one row for each acceptance
  criterion of that issue), `Refs #N` when it only relates to one, or `Task: none, <reason>`.
- A pull request that changes `.github/workflows/` or `.github/actions/` carries no closing keyword:
  write `Task: none, <reason>`, with `Refs #N` beside it if you like.
- Contributors never need the project board or the tracker tooling; a maintainer handles both.
- Older commits, docs and tests cite legacy board ids such as `VQR-0001`. `just issue <ID>` prints
  the issue each one became; `.github/tracker/id-map.json` is the map.

## No identifiers in issues or pull requests

This is a public repository, so no real account identifier or personal data goes in an issue, pull
request, comment or tracked file: no phone numbers, SIP URIs, MAC addresses, host or handset names,
lab IP addresses, handset serials, Grafana stack hostnames or ids, tenant or account IDs,
credentials, or capture payloads containing any of those. Write the shape, not the instance: "the
second handset", "the collector host", `<dialect>/<sender>/<report>`. Aggregate counts, timings,
metric names, CI run ids, commit SHAs and structural findings are fine, and so are SIP `Call-ID`
values: they are per-call hashes that identify nothing.

Use the same placeholder vocabulary as `archive/README.md` (`HOST-A`, `PHONE-A`, `PHONE-B`). The
privacy sweep check on every pull request matches address and hostname shapes; host and handset
names are the one class you must check by eye.

## Git

`codex/` and `docs/superpowers/` are ignored run scaffolding and never enter history.
