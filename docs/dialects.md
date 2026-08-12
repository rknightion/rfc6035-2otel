# Dialects

The service accepts two body dialects from SIP `PUBLISH` requests with content type
`application/vq-rtcpxr` and event package `vq-rtcpxr`.

| Dialect attribute value | Identification | Meaning |
| --- | --- | --- |
| `standard` | RFC 6035 report framing without both `ToID` and `FromID` | RFC 6035 standard encoding |
| `prestandard` | Both `ToID` and `FromID` appear in the body | Measured Poly draft encoding |

The parser recognizes `VQSessionReport`, `VQIntervalReport`, and `VQAlertReport`.
The colon and disposition on the first report line are optional. `LocalMetrics:` and
the alert-example spelling `Metrics:` both mean the local metric block. Folded body
lines are joined before parsing.

Unknown fields are retained in the raw log and as normalized `rfc6035.field.*` log
attributes. That protects the evidence trail when vendor firmware adds a field; it does
not promote a new field to a metric automatically.

The standard and pre-standard paths are both intentionally supported. Do not toggle a
handset merely to make it match the other path: select the dialect actually emitted,
then inspect `rfc6035.report.dialect` in output. A third dialect must be measured and
implemented explicitly rather than guessed from a similar field list.

RFC 3611 uses `127` as an unavailable sentinel for several 8-bit measures. Treat a
reported value as an observation only according to the parser's current conversion
contract; do not interpret a missing or unavailable source value as a zero-quality call.
