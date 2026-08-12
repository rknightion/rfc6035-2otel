package vqreport

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParse(f *testing.F) {
	for _, name := range []string{
		"standard-2026-08-12.txt",
		"prestandard-2026-08-12.txt",
	} {
		capture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "real", name))
		if err != nil {
			f.Fatalf("read required real capture %s: %v", name, err)
		}
		f.Add(capture[bodyStart(capture):])
	}

	// RFC 6035 permits the report disposition and colon to be absent, uses
	// three report types, and contains an example that spells the local block
	// as Metrics rather than LocalMetrics. Keep all of those grammar paths in
	// the seed corpus.
	for _, report := range []string{
		"VQSessionReport: CallTerm\r\nCallID: rfc-session\r\nLocalMetrics:\r\nQualityEst: MOSLQ=4.1 RLQ=93\r\n",
		"VQIntervalReport\r\nCallID: rfc-interval\r\nLocalMetrics:\r\nDelay: RTD=25 IAJ=3\r\n",
		"VQAlertReport\r\nCallID: rfc-alert\r\nMetrics:\r\nPacketLoss: NLR=2.0 JDR=1.0\r\n",
	} {
		f.Add([]byte(report))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		fromBytes, bytesErr := Parse(input)
		fromString, stringErr := Parse(string(input))
		if (bytesErr == nil) != (stringErr == nil) {
			t.Fatalf("input representations disagree: bytes=%v string=%v", bytesErr, stringErr)
		}
		if bytesErr != nil {
			if !errors.Is(bytesErr, ErrUnrecognizedDialect) {
				t.Fatalf("unexpected parse error: %v", bytesErr)
			}
			return
		}
		if fromBytes.Dialect == Unknown || fromBytes.ReportType == "" {
			t.Fatalf("successful parse lacks identity: %#v", fromBytes)
		}
		if fromBytes.Dialect != fromString.Dialect || fromBytes.ReportType != fromString.ReportType || fromBytes.CallID != fromString.CallID {
			t.Fatalf("input representations produced different identities: bytes=%#v string=%#v", fromBytes, fromString)
		}
	})
}
