# Configuration

Configuration precedence is built-in defaults, optional YAML supplied by `--config`,
then `RFC6035_2OTEL_` environment variables. YAML is strict: an unknown key fails
startup. An unknown prefixed environment variable also fails startup.

`config.example.yaml` is the complete annotated configuration. No default OTLP endpoint
exists, so a running listener always needs an explicit endpoint.

## Listener and retransmission handling

```yaml
listen:
  address: "0.0.0.0"
  port: 5060
dedupe_window: "32s"
```

`listen.address` must be non-empty; `listen.port` must be in `1..65535`.
`dedupe_window` must be positive. The default `32s` is approximately SIP Timer J and
deduplicates accepted reports by `Call-ID` and `CSeq`.

## OTLP

```yaml
otlp:
  endpoint: "https://collector.example.test"
  protocol: "http"
  headers: {}
```

`protocol` is `http` or `grpc`. With HTTP, the service appends `/v1/metrics` and
`/v1/logs` to `endpoint` (and replaces an already-supplied one of those paths). With
gRPC, use `host:port`, or an `http://`/`https://` URL containing only host and optional
port. HTTP gRPC URLs select an insecure transport; HTTPS and a bare endpoint use TLS.

Headers are passed to both OTLP exporters. Put credentials in environment variables or
your deployment platform's secret mechanism, not in a committed YAML file.

## Sender registry

```yaml
senders:
  - address: "10.0.0.139"
    name: "office"
```

Each address and name must be non-empty and unique. A configured address maps to its
name. Every unconfigured address maps to the single `unknown` value. This is a
cardinality control: never derive metric sender names from IP addresses, SIP identities,
or call IDs.

With `n` configured names, the sender-name metric cardinality is at most `n + 1`
because `unknown` is always the only fallback. The configuration does not block an
unknown sender; it bounds its telemetry label.

## Logs and resource identity

```yaml
log:
  level: "info"
service:
  name: "rfc6035-2otel"
  version: "dev"
```

`log.level` is `debug`, `info`, `warn`, or `error`. `service.name` and
`service.version` must be non-empty and become OpenTelemetry resource attributes. The
runtime also sets `service.instance.id` from the hostname, falling back to `unknown`.
These explicit service values override the same keys from the standard OpenTelemetry
environment; all other `OTEL_RESOURCE_ATTRIBUTES` values are retained.
