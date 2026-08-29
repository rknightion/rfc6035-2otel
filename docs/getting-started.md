# Getting started

The listener needs a UDP port reachable from the reporting endpoints and an OTLP
metrics-and-logs destination. Start from the tracked example configuration:

```sh
workdir=$(mktemp -d)
cp config.example.yaml "$workdir/config.yaml"
```

Set `otlp.endpoint` to the base endpoint of a collector or backend, then retain only
the sender addresses that are allowed to report. Sender names are stable metric values;
they must be a small, explicit registry.

## Run a local binary

Build and inspect the version:

```sh
just build
./bin/rfc6035-2otel --version
```

Start the binary with `--config config.yaml` after setting a reachable OTLP endpoint.
The process binds `0.0.0.0:5060` by default. A clean shutdown on `SIGINT` or `SIGTERM`
flushes its OTLP providers.

## Container and Helm

The published image is `ghcr.io/rknightion/rfc6035-2otel`; use a release tag in
production, mount configuration read-only, and expose UDP port 5060. The OCI chart is
`oci://ghcr.io/rknightion/charts/rfc6035-2otel`. Its default Service is a UDP
`LoadBalancer` and requests source-address preservation with
`externalTrafficPolicy: Local`. Confirm that the target provider supports UDP load
balancers and that this locality trade-off has enough ready endpoints. The chart README
documents values, including sender configuration and secret handling for OTLP headers.

## Liveness probe

The `--healthcheck <listener-host>:5060` option sends a SIP `OPTIONS` probe to a
running listener and expects its deliberate `405 Method Not Allowed` response. It checks
UDP reachability and the SIP response path; it does not prove that an OTLP backend
received telemetry.
