# Loop: rfc6035-2otel
tier: guarded
gate: just check
ci-required: ci-success
release-on-push: yes
deploy-on-push: yes
receiver: https://loopwatch.m7kni.com
grafana-stack: robknight

Public repository: no phone number, SIP URI, MAC, host or handset name, lab IP, handset serial,
Grafana stack hostname or ID, tenant or account ID, credential or capture payload in any tracked
file, `backlog/` included. Use the placeholder vocabulary in `archive/README.md` (`HOST-A`,
`PHONE-A`). AGENTS.md carries the leak sweep to run over `backlog/`. Run `just` with stdin from
`/dev/null`. `just ci` adds the cross-compilation snapshot and the image build.

## Credentials

- None held by the loop. No recipe is confirm-gated.

## Traps

- A push to `main` touching `grafana/**`, `dashboards/` or `alerts/` runs `grafana-sync`. Those
  directories and the generated signal catalog in `docs/signals.md` are built from `grafana/`
  builders and `spec/signal-catalog.json`; never hand-edit them, edit the input and run `just gen`.
- The public signal contract is frozen: a release cannot rewrite it.
- `just check` includes `fuzz` and `vuln`, so it is the full CI `build-test` leg.
- The parser handles both the standard dialect and the measured Poly pre-standard dialect; a fixture
  for one does not prove the other.

## Mutexes

- One `grafana-sync` writes the stack at a time and is never cancelled mid-write.
