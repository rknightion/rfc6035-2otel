# Frozen protocol contract — RFC 6035 vq-rtcpxr over SIP

Status: frozen 2026-08-12. Derived from **RFC 6035**, **RFC 3611**, **RFC 3903** and **RFC 3261**,
cross-checked against a working reference implementation (Python, 25 passing tests) and against live
Poly Edge E350 handsets on firmware `8.6.0.1321`.

## ⚠️ Provenance — read this before trusting the field list

**No real Poly wire capture existed when this was written.** The transport and response half of this
contract is proven — a live handset fleet is configured to send to a collector and the reference
implementation answers them. The **body field list below comes from the RFCs, not from a measured
Poly artifact.**

That distinction matters because a guessed schema compiles: a parser written against a plausible
shape passes its own synthetic fixtures and is silently wrong about fields it never saw. Two rules
follow, and they are binding:

1. **Where a real capture and this document disagree, the capture wins.** Do not "correct" working
   code to match this file.
2. **Capturing real samples of both dialects is lane W1 of the first wave.** Nothing downstream may
   claim a field is proven until it appears in a capture.

## Transport

| Property | Value |
|---|---|
| Protocol | SIP over **UDP only** |
| Default port | `5060` |
| Method | `PUBLISH` |
| Content-Type | `application/vq-rtcpxr` |
| Event package | `vq-rtcpxr` (RFC 6035 §4.2) |
| Reply required | `SIP/2.0 200 OK`, RFC 3261-conformant |

TCP and TLS are **out of scope**. The Poly Edge E350 exposes no transport selector for the collector
(`voice.qualityMonitoring.collector.server.1.transport` does not exist — it returns in
`InvalidParams`), so UDP is all these handsets can emit.

### The 200 OK is not optional and not cosmetic

The phone retries on no answer and gives up after its retry budget, so a malformed response loses
data silently. Required handling:

- `Via` — echo verbatim, adding `received=` and honouring `rport` (RFC 3581) when present.
- `From` — echo verbatim including its tag.
- `To` — echo, **adding a `tag`** if the request had none.
- `Call-ID`, `CSeq` — echo verbatim.
- `SIP-ETag` — RFC 3903 requires one on a 200 to a PUBLISH.
- `Expires` — grant, never longer than requested.
- `Content-Length: 0`.

Rejections: `405` for a non-PUBLISH method, `415` for a wrong content type, `489` for a wrong event
package.

## Body grammar

`MMDD`-style framing does not apply here — this is not the phone's syslog format. The body is
line-oriented `Key: value` with space-separated `NAME=value` tokens on metric lines, per RFC 6035
§4.6 and the RFC 3611 metric names.

```
VQSessionReport: CallTerm
CallID: <call id>
LocalID: <sip uri>
RemoteID: <sip uri>
OrigID: <sip uri>
LocalAddr: IP=<ip> PORT=<port> SSRC=<ssrc>
RemoteAddr: IP=<ip> PORT=<port> SSRC=<ssrc>
LocalMetrics:
Timestamps: START=<iso8601> STOP=<iso8601>
SessionDesc: PT=<payload type> PD=<codec> SR=<rate> FD=<..> FO=<..> FPP=<..> PPS=<..> PLC=<..> SSUP=<..>
JitterBuffer: JBA=<..> JBR=<..> JBN=<..> JBM=<..> JBX=<..>
PacketLoss: NLR=<pct> JDR=<pct>
BurstGapLoss: BLD=<..> BD=<..> GLD=<..> GD=<..> GMIN=<..>
Delay: RTD=<ms> ESD=<ms> SOWD=<ms> IAJ=<ms> MAJ=<ms>
QualityEst: RLQ=<..> RCQ=<..> MOSLQ=<..> MOSCQ=<..> QoEEstAlg=<..>
```

Report types: `VQSessionReport`, `VQIntervalReport`, `VQAlertReport`.

## Traps — each defeats a specific wrong-but-plausible implementation

1. **The HCOLON is optional on the report-type line.** RFC 6035 §4.6.1's ABNF is
   `SessionReport = "VQSessionReport" [ HCOLON "CallTerm" ] CRLF`, so a bare `VQSessionReport` with
   no colon is legal. A parser requiring the colon drops the report type and looks like it worked.
   *This was a real bug in the reference implementation, caught only by a test written from the ABNF.*
2. **RFC 6035 contradicts itself on the local metrics block.** §4.6.1 specifies `LocalMetrics:`;
   the alert example in §4.7.4 writes a bare `Metrics:`. Accept both; treat `Metrics:` as local.
3. **Absent is not zero.** A metric the phone omits must be absent from the output, never defaulted.
   Emitting `0` for a missing MOS produces a dashboard showing catastrophic call quality that did not
   happen.
4. **RFC 3611 uses `127` as an "unavailable" sentinel** for several 8-bit metrics. It is not a value.
   The reference implementation deliberately does *not* interpret it and passes values through as
   strings; this project must decide explicitly and document the choice.
5. **Lines fold.** RFC 3261 header folding applies to the body's `Key: value` lines. A line-split
   parser that ignores continuation lines silently truncates metrics.
6. **Unknown keys must be retained, not discarded.** Poly emits non-RFC tokens (`SCS`, `extRFactor`,
   `X-` prefixed lines). Dropping them loses vendor data and hides dialect drift.
7. **Retransmissions are expected.** UDP plus SIP retry means the same report arrives more than once.
   De-duplicate on `Call-ID` + `CSeq` over roughly a Timer J window (~32s) or the metrics double-count.

## The two dialects — and why both are in scope

`voice.qualityMonitoring.rfc6035.enable` on the Poly handset selects the encoding:

| Value | Encoding |
|---|---|
| `1` | RFC 6035 standard — what the grammar above describes |
| `0` (**Poly default**) | Poly's **pre-standard draft** encoding — a different dialect |

**Most Poly fleets in the wild are on the default `0`** and their operators will not know to change
it. Supporting only the standard would make the project useless to most of its potential users, so
both are in scope, auto-detected from the body.

The pre-standard grammar is **not documented here because no capture of it existed at freeze time.**
It must be measured, not guessed — see lane W1.

### Live sample source — one handset per dialect

A two-handset Poly Edge E350 fleet is deliberately split so both dialects are emitted concurrently:

| Handset | Address | VLAN | `rfc6035.enable` | Dialect |
|---|---|---|---|---|
| `deskie` | `10.0.0.139` | LAN | `1` | RFC 6035 standard |
| `extra` | `10.0.50.175` | IOT (vlan50) | `0` | Poly pre-standard |

Both send to `10.0.0.5:5060/udp`. This is the intended source of the real captures, and it is why a
second instance is available at all — per the "a second sample beats a bigger first one" rule, two
instances from different circumstances catch a class of bug that volume never will.
