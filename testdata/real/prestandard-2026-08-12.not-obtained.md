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

Resume by capturing on the handset's own interface (Poly Lens device PCAP, if separately authorised)
or by temporarily moving `extra` to Ethernet/LAN as described in the Wave 1 goal. Until then, code may
detect a structurally pre-standard body only to reject it explicitly as unsupported; it must not claim
pre-standard parsing support.
