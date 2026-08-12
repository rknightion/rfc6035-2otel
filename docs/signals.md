# Signals

This is the public signal contract. The metric names, kinds, units, Prometheus names,
and attributes match `spec/signal-catalog.json`. OTLP backends may render names and
units in their own query syntax; the Prometheus column is the canonical Prometheus
translation, including counter `_total` and seconds suffixes.

## Complete metric catalog

<!-- BEGIN GENERATED SIGNAL CATALOG -->

| OTLP metric | Prometheus name | Kind | Unit | Attributes |
| --- | --- | --- | --- | --- |
| `rfc6035.call.mos_lq` | `rfc6035_call_mos_lq` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.mos_cq` | `rfc6035_call_mos_cq` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.r_factor_lq` | `rfc6035_call_r_factor_lq` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.r_factor_cq` | `rfc6035_call_r_factor_cq` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.packet_loss` | `rfc6035_call_packet_loss` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.discard_rate` | `rfc6035_call_discard_rate` | histogram | `1` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.round_trip_delay` | `rfc6035_call_round_trip_delay_seconds` | histogram | `s` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.one_way_delay` | `rfc6035_call_one_way_delay_seconds` | histogram | `s` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name` |
| `rfc6035.call.jitter` | `rfc6035_call_jitter_seconds` | histogram | `s` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, `rfc6035.sender.name`, `rfc6035.jitter.kind` |
| `rfc6035_2otel.build_info` | `rfc6035_2otel_build_info` | gauge | `1` | `service.version`, `vcs.ref.head.revision`, `rfc6035_2otel.build.date`, `rfc6035_2otel.build.go_version` |
| `rfc6035_2otel.datagrams` | `rfc6035_2otel_datagrams_total` | counter | `{datagram}` | `rfc6035_2otel.datagram.outcome` |
| `rfc6035_2otel.reports` | `rfc6035_2otel_reports_total` | counter | `{report}` | `rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.sender.name` |
| `rfc6035_2otel.parse_errors` | `rfc6035_2otel_parse_errors_total` | counter | `{error}` | `error.type` |
| `rfc6035_2otel.duplicates` | `rfc6035_2otel_duplicates_total` | counter | `{report}` | `rfc6035.sender.name` |
| `rfc6035_2otel.responses` | `rfc6035_2otel_responses_total` | counter | `{response}` | `rfc6035.response.status_code` |
| `rfc6035_2otel.export_failures` | `rfc6035_2otel_export_failures_total` | counter | `{failure}` | `rfc6035_2otel.signal` |
| `rfc6035_2otel.dedupe_cache.usage` | `rfc6035_2otel_dedupe_cache_usage` | updowncounter | `{entry}` | none |
| `rfc6035_2otel.report.process.duration` | `rfc6035_2otel_report_process_duration_seconds` | histogram | `s` | `rfc6035.report.dialect` |

<!-- END GENERATED SIGNAL CATALOG -->

## Call-quality metric values

Each present source observation records one synchronous histogram. They describe
completed-call observations, not cumulative totals. All call metrics carry
`rfc6035.report.dialect`, `rfc6035.report.type`, `rfc6035.report.side`, and
`rfc6035.sender.name`; `rfc6035.call.jitter` additionally carries
`rfc6035.jitter.kind`.

| OTLP metric | Wire field and wire unit | Exported value |
| --- | --- | --- |
| `rfc6035.call.mos_lq` | `MOSLQ`, score | unchanged score, unit `1` |
| `rfc6035.call.mos_cq` | `MOSCQ`, score | unchanged score, unit `1` |
| `rfc6035.call.r_factor_lq` | `RLQ`, score | unchanged score, unit `1` |
| `rfc6035.call.r_factor_cq` | `RCQ`, score | unchanged score, unit `1` |
| `rfc6035.call.packet_loss` | `NLR`, percent | percent ÷ 100, unit fraction `1` |
| `rfc6035.call.discard_rate` | `JDR`, percent | percent ÷ 100, unit fraction `1` |
| `rfc6035.call.round_trip_delay` | `RTD`, ms | ms ÷ 1000, seconds |
| `rfc6035.call.one_way_delay` | `SOWD`, ms | ms ÷ 1000, seconds |
| `rfc6035.call.jitter` | `IAJ` or `MAJ`, ms | ms ÷ 1000, seconds |

For jitter, `rfc6035.jitter.kind` is `interarrival` for `IAJ` and `mean_absolute` for
`MAJ`. `rfc6035.report.side` is `local` or `remote`. Report type is one of
`VQSessionReport`, `VQIntervalReport`, or `VQAlertReport`; the parser owns dialect
values (`standard`, `prestandard`).

**Absent is not zero.** If the report lacks one of these wire fields, the exporter
records no observation. A zero-valued metric would falsely claim a measurement and must
not be used to represent a missing MOS, loss, delay, or jitter field.

## Collector self-observability values

The bounded self-observability value registries are:

- `rfc6035_2otel.datagram.outcome`: `accepted`, `rejected`, `malformed`.
- `error.type`: `invalid_input`, `malformed_datagram`, `malformed_sip`,
  `unrecognized_dialect`, `invalid_value`, `export_failed`.
- `rfc6035.response.status_code`: `200`, `405`, `415`, `489`.
- `rfc6035_2otel.signal`: `metrics`, `logs`.

## Sender cardinality and logs

`rfc6035.sender.name` is sourced only from `senders[].name`. For `n` configured
senders, its maximum metric cardinality is `n + 1`: every unmatched address collapses
to `unknown`. A call ID, SIP URI, source address, source port, and parsed field values
never become metric attributes.

Each accepted report emits one native OTLP log with event name
`rfc6035.report.received`, severity `INFO`, and the raw report as its body. Its stable
attributes are `rfc6035.report.call_id`, `rfc6035.report.dialect`,
`rfc6035.report.type`, `rfc6035.sender.name`, `client.address`, `client.port`,
`network.transport=udp`, and `network.protocol.name=sip`. Parsed fields are included as
normalized `rfc6035.field.*` log attributes, except credential-like field names.
