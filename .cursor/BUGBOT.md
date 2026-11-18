# RFC 6035 collector review guide

## What this is

This Go collector receives SIP voice-quality reports over UDP and exports quality measurements and native logs through OpenTelemetry. `cmd/rfc6035-2otel/` connects the listener, parser, configuration and telemetry.

## Invariants

- Preserve the SIP PUBLISH shape and datagram bounds in `internal/sip`. The listener must acknowledge and deduplicate valid retransmissions by Call-ID and CSeq while malformed datagrams leave it running.
- Keep both RFC and supported pre-standard report forms in `internal/vqreport`. Preserve the raw report, structured field mapping, and missing-value semantics alongside the typed measurements.
- Keep sender metrics bounded to configured names and the `unknown` fallback. Keep call identifiers, addresses and raw report fields in the documented log surface. Preserve one native log per accepted report and the credential-like field-name exclusion.
- Keep metric, field, unit and label compatibility. Update the owning `spec/signal-catalog.json` and regenerate dependent assets with `just gen` when the signal contract changes.

## Where to look hardest

- Check SIP reply and transaction changes against `internal/sip/listener_test.go` and `response_test.go`. Check parser changes against both dialect fixtures and fuzz coverage.
- Follow changes to data projection through `internal/otelexport` and the end-to-end mapping tests in `cmd/rfc6035-2otel/`. Preserve intentional log-field and metric-label boundaries.
- Check `internal/config` and Helm changes for strict config decoding, environment precedence, sender validation and runtime secret injection. Review the chart templates and their values together.

## Checks already covered

Rely on the existing formatting, lint, vet, race, module tidy, vulnerability, parser fuzz and generated-asset checks for their covered findings. CI also runs workflow lint, Zizmor, dependency review and container checks.

## Verification

Run `just setup` and `just check </dev/null>`. CI additionally builds a release snapshot and container, and validates Helm. Use the captured reports and focused parser/export tests for protocol and telemetry claims. Use designated live readback for deployment claims.
