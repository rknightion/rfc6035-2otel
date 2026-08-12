// Package selfobs records the collector's fixed, low-cardinality operational
// signals. It intentionally owns no SIP, parser, or OTLP-export implementation
// details, so those packages can depend on Recorder without depending on one
// another.
//
// error.type is a closed registry: invalid_input, malformed_datagram,
// malformed_sip, unrecognized_dialect, invalid_value, and export_failed. New
// error classes require an explicit package change; arbitrary error messages
// must never become metric attributes. Sender names are likewise supplied as a
// finite constructor registry, so raw addresses, SIP URIs, call IDs, and other
// unbounded values cannot enter a metric.
package selfobs
