# rfc6035-2otel

`rfc6035-2otel` receives SIP `PUBLISH` voice-quality reports over UDP and exports
their completed-call observations as OpenTelemetry metrics and logs over OTLP. It is
for operators of endpoints that emit RFC 6035 VQ reports, including the two report
dialects documented here.

The listener answers every accepted `PUBLISH` with a SIP `200 OK`, including the
required `SIP-ETag`. It rejects a wrong method, content type, or event package with a
standards-appropriate SIP response. UDP retransmissions are deduplicated on `Call-ID`
plus `CSeq` for a configurable window, so completed-call histograms are not
double-counted.

## Signal model

The service emits two intentionally different shapes:

- `rfc6035.call.*` histograms hold present voice-quality measurements. They use only
  bounded attributes: report dialect, report type, configured sender name, side, and
  jitter kind where relevant.
- Every accepted report also creates one native OTLP log, `rfc6035.report.received`.
  The body retains the raw report and its attributes retain the call ID, sender network
  endpoint, and parsed fields. These fields are not metric dimensions.

The [signal reference](signals.md) is the complete contract. In particular, missing
wire fields produce no metric observation: **absent is not zero**.

## Start here

1. Follow [Getting started](getting-started.md) for a local binary, container, or Helm
   deployment.
2. Configure a bounded sender registry and OTLP target in
   [Configuration](configuration.md).
3. Read [Dialects](dialects.md) before assuming a handset's wire format.
4. Use [Troubleshooting](troubleshooting.md) when a sender retries or output is absent.
5. Run a real non-Poly endpoint test with [Compatibility](compatibility.md).
6. Read [RFC 3611 RTCP-XR boundary](rtcp-xr.md) before planning binary RTCP-XR capture.

The service is UDP-only. It does not accept SIP over TCP or TLS, expose a Prometheus
endpoint, make outbound calls to endpoints, or emit traces.
