# Loop: rfc6035-2otel
tier: guarded
gate: just check
ci-required: ci-success
release-on-push: yes
deploy-on-push: yes
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
- A metric or log field is proven by querying the live Grafana stack (positive and negative), not by a
  unit test: an explicit OTel gauge unit of `1` makes the SDK append `_ratio` to the name. Any lane
  touching `internal/otelexport`, `spec/signal-catalog.json` or the dashboards closes with a real query.
- A wrong name, unit, attribute or log field in a published release is fixed by the next patch, never
  a retag.
- Do not divide `NLR`/`JDR` by 100 and do not invent a unit: they are exported as UCUM `%` as received.
  RFC 3611's `127` sentinel is nil on the typed field, and the literal survives in the lossless log
  fields. A lane that "normalises" either reverses a decision.
- `internal/vqreport` alone owns the dialect enum; the exporter passes the parser-owned value through.
- Fixtures are labelled real or synthetic. Never promote a synthetic fixture to evidence of what a
  device emits.
- Before reporting a negative from telemetry, state which stream was queried and why it would carry
  the signal. `rfc6035-2otel` exports OTLP directly and writes nothing to a syslog stream.
- The collector host's `senders:` mapping exists only on the host, with no durable source: verify it
  is present before drawing a conclusion from `unknown` senders.
- Releases stay pre-1.0 until a non-Poly RFC 6035 sender is observed and the pre-standard grammar is
  seen from a second Poly model or firmware.
- Test-side timing races get a bounded wait in the assertion, not serialised production code. A
  transient `curl 56` or HTTP 503 on a tool download is infrastructure: rerun once and say so.
- The parser handles both the standard dialect and the measured Poly pre-standard dialect; a fixture
  for one does not prove the other.

## Mutexes

- Ownership is per package: `internal/sip`, `internal/vqreport`, `internal/otelexport`,
  `internal/selfobs`, `internal/config`, `charts/` with `deploy/`, `.github/workflows/`, `docs/` with
  `spec/`. `cmd/rfc6035-2otel` and any cross-cutting rename are the wiring pass's, never a parallel lane's.
- The live handsets and the collector-host deployment (a second lane placing calls invalidates the
  first's capture window), the Grafana Cloud stack, and placing real calls are exclusive and root only.
  Call origination is a separate grant from having the handsets in scope.
- One `grafana-sync` writes the stack at a time and is never cancelled mid-write.
