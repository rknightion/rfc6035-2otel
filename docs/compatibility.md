# Compatibility

`rfc6035-2otel` accepts a SIP `PUBLISH` carrying
`application/vq-rtcpxr`. This is an RFC 6035 reporting interface, not a generic
RTCP-XR receiver: an endpoint that sends RFC 3611 RTCP-XR packets during media does
not thereby send a report to this collector.

## Non-Poly compatibility test

Linphone Desktop is the preferred non-Poly test sender. Configure its SIP account to
register with the PBX, set its RFC 6035 voice-quality-report collector to exactly:

```
sip:10.0.0.5:5060;transport=udp
```

The collector is not a SIP registrar or peer. Place and complete a real call through
the PBX; registration, a call attempt, or locally generated media alone is not proof.
The endpoint must send the completed-call `PUBLISH` over UDP and receive the collector's
`200 OK`.

Treat the test as successful only when the live telemetry also shows the report:

- locate an `rfc6035.report.received` log from the Linphone sender and inspect its raw
  report body, dialect, source endpoint, and parsed fields;
- confirm that `rfc6035_2otel_reports_total` increased for the configured sender; and
- confirm that only metrics for present source fields gained observations. Missing
  delay, MOS, loss, or jitter fields must remain absent rather than appearing as zero.

Add Linphone's stable IP address to `senders` before the call so the metric label is a
configured sender name, not `unknown`. See [Configuration](configuration.md) for the
bounded sender registry and [Signals](signals.md) for metric names and units.

Linphone is a compatibility candidate, not a blanket promise about every build or
softphone. Verify the actual wire report after every client or PBX change. Do not infer
RFC 6035 `PUBLISH` support from generic RTCP, RTCP-XR, media-quality, or call-statistics
support; those interfaces can exist without this SIP reporting path.
