// Package vqreport parses RFC 6035 vq-rtcpxr report bodies.
//
// Numeric fields use pointers: nil means the phone did not provide a usable
// observation. RFC 3611's unavailable value 127 is treated as nil only for
// registered RFC 3611 fields (currently RERL); its literal field remains in
// Report.Fields for lossless logs and vendor analysis. All unknown fields and
// physical input lines are retained. Poly's measured pre-standard draft uses
// paired ToID/FromID fields instead of the standard identity field names; that
// structural pair selects the Prestandard dialect without a configuration
// flag. Bodies with no recognised report line return ErrUnrecognizedDialect.
package vqreport
