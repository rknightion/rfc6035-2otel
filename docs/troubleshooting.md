# Troubleshooting

## The endpoint retries or reports no successful publish

Check UDP reachability first. The listener accepts only SIP `PUBLISH` with
`Content-Type: application/vq-rtcpxr` and `Event: vq-rtcpxr`:

| SIP response | Meaning |
| --- | --- |
| `200 OK` | Accepted; response includes `SIP-ETag` and a granted `Expires` value. |
| `405 Method Not Allowed` | The request method was not `PUBLISH`. |
| `415 Unsupported Media Type` | The content type was not `application/vq-rtcpxr`. |
| `489 Bad Event` | The event package was not `vq-rtcpxr`. |

Use the built-in `--healthcheck <listener-host>:5060` probe from the listener host. It
intentionally expects `405`, so exit status zero verifies a live UDP listener and
response path rather than a successful report upload.

## Accepted reports appear only once

This is normal for a retransmission. The listener serves the cached response and omits a
second handler call when `Call-ID` and `CSeq` repeat inside `dedupe_window`. Inspect
`rfc6035_2otel.duplicates` and adjust only after understanding endpoint retry behaviour.

## A quality metric is missing

Missing is meaningful. The exporter does not synthesize a zero for an omitted wire
field, so query for observations rather than assuming a zero-valued series. Inspect the
corresponding `rfc6035.report.received` log, its `rfc6035.field.*` attributes, and the
raw log body to distinguish an absent source field from a query issue.

Packet loss and discard rate are converted from wire percent to unit fraction; delay and
jitter are converted from wire milliseconds to seconds. Check those units before setting
thresholds. See [Signals](signals.md).

## Metrics or logs are absent from the backend

Verify `otlp.endpoint`, protocol, TLS expectation, and headers. HTTP endpoints receive
`/v1/metrics` and `/v1/logs`; gRPC accepts only host:port or an HTTP(S) URL without a
path. The process records `rfc6035_2otel.export_failures` when an exporter path reports
a failure. A successful health check does not test OTLP delivery.

## An expected sender label is `unknown`

`senders[].address` is compared to the UDP source address, not a hostname, SIP URI, or
address embedded in the report. Add the exact reporting IP and a stable name to YAML,
then restart. Do not use dynamic sender names to work around the fallback; that defeats
the cardinality bound.
