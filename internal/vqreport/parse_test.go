package vqreport

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRFCSessionReportTraps(t *testing.T) {
	got, err := Parse("VQSessionReport\nCallID: call-1\nLocalAddr: IP= 192.0.2.1 PORT=5004 SSRC=7\nMetrics:\nSessionDesc: PT=8 PD=G.711 A-Law PPS=50\nQualityEst: MOSLQ=4.1\nX-Vendor: SCS=99 extRFactor=101\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.Dialect != Standard || got.ReportType != "VQSessionReport" {
		t.Fatalf("identity = %#v", got)
	}
	if got.CallID != "call-1" || got.LocalAddress == nil || got.LocalAddress.IP.String() != "192.0.2.1" {
		t.Fatalf("identity/address = %#v", got)
	}
	if got.LocalAddress.Port == nil || *got.LocalAddress.Port != 5004 {
		t.Fatalf("port = %#v", got.LocalAddress.Port)
	}
	if got.LocalMetrics.Codec == nil || *got.LocalMetrics.Codec != "G.711 A-Law" {
		t.Fatalf("codec = %#v", got.LocalMetrics.Codec)
	}
	if got.LocalMetrics.MOSLQ == nil || *got.LocalMetrics.MOSLQ != 4.1 {
		t.Fatalf("MOSLQ = %#v", got.LocalMetrics.MOSLQ)
	}
	if got.LocalMetrics.RLQ != nil {
		t.Fatal("absent metric became present")
	}
	if len(got.Unknown["xvendorscs"]) != 1 || len(got.Unknown["xvendorextrfactor"]) != 1 {
		t.Fatalf("unknown = %#v", got.Unknown)
	}
}

func TestParseFoldedLines(t *testing.T) {
	got, err := Parse([]byte("VQSessionReport: CallTerm\r\nCallID: call-2\r\nLocalMetrics:\r\nQualityEst: MOSLQ=\r\n 4.2 RLQ=93\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.LocalMetrics.MOSLQ == nil || *got.LocalMetrics.MOSLQ != 4.2 || got.LocalMetrics.RLQ == nil || *got.LocalMetrics.RLQ != 93 {
		t.Fatalf("folded metrics = %#v", got.LocalMetrics)
	}
}

func TestParseSentinelRetainedInRaw(t *testing.T) {
	got, err := Parse("VQSessionReport\nSignal:RERL=127\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.LocalMetrics.RERL != nil {
		t.Fatalf("RERL = %#v, want nil", got.LocalMetrics.RERL)
	}
	if !hasField(got.Fields, "Signal.RERL", "127") {
		t.Fatalf("raw fields lost RERL sentinel: %#v", got.Fields)
	}
}

// Stable report identity is parser-owned; SIP retransmission caching belongs
// to the listener package, not here.
func TestParseExtractsCallIdentity(t *testing.T) {
	got, err := Parse("VQSessionReport\nCallID: stable-call-id\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.CallID != "stable-call-id" {
		t.Fatalf("CallID = %q", got.CallID)
	}
}

func TestParseRealStandardCapture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "real", "standard-2026-08-12.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(body[bodyStart(body):])
	if err != nil {
		t.Fatal(err)
	}
	if got.Dialect != Standard || got.ReportType != "VQSessionReport" || got.CallID != "151a6cd90d16552a7ef51db8b1139733" {
		t.Fatalf("report = %#v", got)
	}
	if got.LocalAddress == nil || got.LocalAddress.IP.String() != "10.0.0.139" {
		t.Fatalf("local = %#v", got.LocalAddress)
	}
	if got.LocalMetrics.Codec == nil || *got.LocalMetrics.Codec != "G.711 A-Law" {
		t.Fatalf("codec = %#v", got.LocalMetrics.Codec)
	}
	if got.LocalMetrics.RERL != nil || !hasField(got.Fields, "Signal.RERL", "127") {
		t.Fatalf("sentinel/raw = %#v", got)
	}
}

func TestParseUnrecognizedDialect(t *testing.T) {
	_, err := Parse("DraftVQReport: something\nCallID: call-3\n")
	if !errors.Is(err, ErrUnrecognizedDialect) {
		t.Fatalf("error = %v", err)
	}
}

func TestPrestandardEnumIsFrozenButUnmeasured(t *testing.T) {
	if Prestandard.String() != "prestandard" {
		t.Fatal("prestandard enum changed")
	}
}

func hasField(fields []Field, key, value string) bool {
	for _, field := range fields {
		if field.Key == key && field.Value == value {
			return true
		}
	}
	return false
}

func bodyStart(b []byte) int {
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '\n' && b[i+1] == '\n' {
			return i + 2
		}
	}
	return 0
}
