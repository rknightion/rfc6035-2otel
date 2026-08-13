# RFC 3611 RTCP-XR boundary

Binary RFC 3611 RTCP-XR belongs in a future sibling receiver,
`rtcp-xr-2otel`, not in `rfc6035-2otel`. This service is deliberately a UDP SIP
`PUBLISH` collector for RFC 6035 report bodies. Combining it with media-plane packet
capture would blur deployment, security, and cardinality boundaries and make a
successful RFC 6035 deployment depend on access to RTP traffic.

## Intended receiver boundary

The sibling starts with offline PCAP decoding. A passive TAP or SPAN deployment is a
later, separately designed capture mode; it must observe RTP/RTCP without becoming a
SIP endpoint, media relay, or packet injector. SRTCP is explicitly out of scope: the
receiver will not collect, request, store, or use encryption keys to decrypt protected
RTCP.

The identity model must remain bounded. Site and collector identity may be configured;
stable endpoint or stream identities require an explicit, finite registry. SSRCs, IP
addresses, ports, CNAMEs, call identifiers, and other per-call values belong in logs or
bounded correlation records, never unbounded metric labels. Unknown identities must
collapse to a fixed value rather than minting a new time series.

## Proof sequence

Before an implementation claims RFC 3611 support, establish the decoding path in this
order:

1. Create or obtain a known-good PCAP and use Wireshark to identify the RTCP-XR packet
   and block types independently.
2. Decode that same capture with Pion RTCP and compare the parsed blocks and values
   against Wireshark.
3. Use PJSIP/PJMEDIA as a separate sender/receiver implementation where it can produce
   the relevant RTCP-XR blocks; capture its traffic and repeat the comparison.
4. Add fixtures only from retained packet evidence and preserve unknown or unsupported
   blocks for inspection rather than inventing values.

This sequence is deliberately evidence-first. Pion, PJSIP/PJMEDIA, and Wireshark can
prove binary RFC 3611 interoperability; none proves that a softphone emits RFC 6035
SIP `PUBLISH`. For that separate control-plane test, see [Compatibility](compatibility.md).
