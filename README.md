# rfc6035-2otel

`rfc6035-2otel` receives **SIP voice-quality reports** — [RFC 6035](https://www.rfc-editor.org/rfc/rfc6035)
`PUBLISH` requests carrying an `application/vq-rtcpxr` body, whose metric syntax comes from
[RFC 3611](https://www.rfc-editor.org/rfc/rfc3611) — and exports them as **OpenTelemetry metrics and
logs** over OTLP.

It is a single Go process with one inbound UDP listener and no Prometheus scrape endpoint.

## Why this exists

VoIP handsets can report per-call quality — MOS, R-factor, jitter, packet loss, round-trip and
one-way delay — but only over SIP. No OpenTelemetry Collector receiver speaks SIP, and the reports
are not a log stream: the phone sends a `PUBLISH` and requires a well-formed `200 OK` in reply, or it
retries and eventually gives up.

This bridges that gap directly to OTLP, with no syslog hop.

## Status

Early development. See `spec/` for the frozen protocol contract this implements.

## Licence

AGPL-3.0. See `LICENSE`.
