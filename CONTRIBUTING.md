# Contributing to rfc6035-2otel

Contributions are welcome. Keep the protocol boundary and telemetry contract explicit:
this project receives UDP SIP `PUBLISH` reports and exports bounded OTLP metrics plus
raw-detail logs.

## Development

Requires the Go version declared by `go.mod`. The local acceptance command is:

```sh
make check
```

It runs formatting, vet, race-enabled tests, module-tidiness verification, and a binary
build. `make build` writes `bin/rfc6035-2otel`; `make test` runs the race detector.

## Changes

- Add a focused failing test before changing parser, SIP response, conversion, dedupe, or
  configuration behaviour.
- Preserve the protocol evidence boundary: do not invent pre-standard fields from
  documentation. A real capture outranks a plausible schema.
- Keep missing report fields absent from output. Never turn them into zero-valued quality
  observations.
- Keep metric attributes bounded. Call IDs, addresses, SIP identities, and arbitrary
  parsed fields belong in logs, not metric dimensions.
- Update `spec/signal-catalog.json` and `docs/signals.md` together for a public signal
  contract change.

Use Conventional Commits (`feat:`, `fix:`, `docs:`, and so on). Breaking changes use
`!` and a `BREAKING CHANGE:` footer. The project is licensed under AGPL-3.0-only.
