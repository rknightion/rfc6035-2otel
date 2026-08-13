# rfc6035-2otel

`rfc6035-2otel` receives RFC 6035 voice-quality reports as SIP `PUBLISH` requests
over UDP and exports OpenTelemetry metrics and logs over OTLP. It supports the standard
and measured Poly pre-standard dialects, has one UDP listener, and no Prometheus scrape
endpoint. Metrics stay bounded while per-call detail remains in logs.

The collector replies to valid reports with SIP `200 OK`; it rejects unsupported methods, content types, and event packages with the applicable SIP response. UDP retransmissions are deduplicated.

## Quick start

```sh
make build
./bin/rfc6035-2otel -version
```

Copy [config.example.yaml](config.example.yaml), set a real OTLP endpoint, and run the
binary with `-config`. The default listener is UDP `0.0.0.0:5060`; do not expose it to
untrusted networks. Configure `senders` with stable names for expected source addresses;
every unmatched source collapses to the single `unknown` metric value.

See the [operator documentation](docs/index.md) for [configuration](docs/configuration.md),
[signals](docs/signals.md), [dialects](docs/dialects.md), [security](docs/security.md),
[troubleshooting](docs/troubleshooting.md), [non-Poly compatibility testing](docs/compatibility.md),
and the [RFC 3611 RTCP-XR boundary](docs/rtcp-xr.md).

## Licence

AGPL-3.0. See [LICENSE](LICENSE).
