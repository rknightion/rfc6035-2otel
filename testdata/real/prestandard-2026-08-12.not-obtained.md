# Poly pre-standard capture disposition — 2026-08-12

No pre-standard wire capture was obtained. The grammar remains unmeasured and must not be inferred
from the standard dialect.

The `extra` Poly Edge E350 at `10.0.50.175` read back with
`voice.qualityMonitoring.rfc6035.enable=0`, session collection enabled, periodic collection disabled,
and collector `10.0.0.5:5060`. Two real calls to the handset's voicemail service were connected and
terminated while `tcpdump` on camden captured every UDP datagram from `10.0.50.175` to
`10.0.0.5:5060`; no matching datagram arrived. The handset's independent syslog stream showed each
attempt enter `Publish Vq-Rtcp-Xr` for approximately 32 seconds and then return to `Idle`, while the
collector received no report.

Follow-up read-only diagnosis compared every live voice-quality setting on `extra` with `deskie`.
The collector address/port, RTCP-XR enable, session/periodic switches, location, alert thresholds,
and IPv4-only network mode are identical; only the deliberate `rfc6035.enable` dialect selector
differs. The same call procedure produces these distinct phone-local outcomes:

```text
extra:  CStatePublishClient::OnEvResponse 480
extra:  CTrans::TimeOut500ms Self Generated 480 Response ... method 'PUBLISH'
deskie: CStatePublishClient::OnEvResponse 200
```

The extra transaction enters `Publish Vq-Rtcp-Xr`, self-times out 32 seconds later, and reports a
self-generated 480. A simultaneous 90-second camden capture records zero packets from
`10.0.50.175`; deskie produces one PUBLISH and one 200 under the same collector configuration. This
narrows the failure to the handset or to a destination selected before traffic reaches camden. It
does **not** prove where pre-standard mode sends the PUBLISH, and the grammar must still not be
inferred.

A read-only Poly Lens `accessPCAPUrl` check returned `NO_AVAILABLE_PCAPS`. Starting a new device PCAP
is an external device mutation that was not granted by the Wave 1 contract, so it was not attempted.

Resume by separately authorising a Poly Lens device PCAP and capturing this exact call-termination
window, by physically moving `extra` to Ethernet/LAN as described in the Wave 1 goal, or by obtaining
the attempted PUBLISH from the SIP proxy selected by the handset. Until then, code may detect a
structurally pre-standard body only to reject it explicitly as unsupported; it must not claim
pre-standard parsing support.
