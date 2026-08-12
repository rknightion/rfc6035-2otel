package main

import (
	"context"
	"net"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/rknightion/rfc6035-2otel/internal/config"
	"github.com/rknightion/rfc6035-2otel/internal/sip"
	"github.com/rknightion/rfc6035-2otel/internal/vqreport"
)

func TestExportReportMapsTypedMetricsAndLosslessFields(t *testing.T) {
	mos := 4.1
	report := vqreport.Report{
		Dialect: vqreport.Standard, ReportType: "VQSessionReport", CallID: "body-call",
		LocalMetrics: vqreport.Metrics{MOSLQ: &mos},
		Fields: []vqreport.Field{
			{Key: "Signal.RERL", Value: "127"},
			{Key: "X-Vendor", Value: "first"},
			{Key: "X-Vendor", Value: "second"},
		},
	}
	got := exportReport(report, sip.Publish{
		Body:   []byte("VQSessionReport\nCallID: body-call\n"),
		CallID: "sip-call", CSeq: "1 PUBLISH",
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.8"), Port: 5060},
	})
	if got.LocalMOSLQ != &mos || got.RemoteMOSLQ != nil {
		t.Fatalf("metric mapping = %#v", got)
	}
	if got.CallID != "body-call" || got.SourceAddress != "192.0.2.8" || got.SourcePort != 5060 {
		t.Fatalf("identity mapping = %#v", got)
	}
	if got.RawReport != "VQSessionReport\nCallID: body-call\n" {
		t.Fatalf("raw report = %q", got.RawReport)
	}
	for key, want := range map[string]string{
		"Signal.RERL": "127", "X-Vendor": "first", "X-Vendor[2]": "second",
		"SIP.Call-ID": "sip-call", "SIP.CSeq": "1 PUBLISH",
	} {
		if got.Fields[key] != want {
			t.Errorf("field %s = %q, want %q", key, got.Fields[key], want)
		}
	}
}

func TestResourceHonorsEnvironmentWithExplicitServiceOverrides(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name=test,service.name=from-env")
	t.Setenv("OTEL_SERVICE_NAME", "also-from-env")
	res, err := newResource(context.Background(), config.Config{
		Service: config.ServiceConfig{Name: "rfc6035-2otel", Version: "0.1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	attrs := make(map[string]attribute.Value)
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}
	for key, want := range map[string]string{
		"deployment.environment.name": "test",
		"service.name":                "rfc6035-2otel",
		"service.version":             "0.1.0",
		"service.instance.id":         hostname(),
		"telemetry.sdk.language":      "go",
	} {
		if got := attrs[key].AsString(); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestParseErrorType(t *testing.T) {
	if got := parseErrorType(vqreport.ErrUnrecognizedDialect); got != "unrecognized_dialect" {
		t.Fatalf("unrecognized dialect = %q", got)
	}
	if got := parseErrorType(vqreport.ErrInvalidInput); got != "invalid_input" {
		t.Fatalf("invalid input = %q", got)
	}
}

func TestOTLPEndpoints(t *testing.T) {
	if got := otlpHTTPURL("https://example.test/otlp/v1/metrics", "logs"); got != "https://example.test/otlp/v1/logs" {
		t.Fatalf("HTTP log endpoint = %q", got)
	}
	for _, test := range []struct {
		raw, endpoint string
		insecure      bool
	}{
		{"collector.example:4317", "collector.example:4317", false},
		{"http://collector.example:4317", "collector.example:4317", true},
		{"https://collector.example:4317", "collector.example:4317", false},
	} {
		endpoint, insecure, err := grpcEndpoint(test.raw)
		if err != nil || endpoint != test.endpoint || insecure != test.insecure {
			t.Fatalf("grpcEndpoint(%q) = %q, %t, %v", test.raw, endpoint, insecure, err)
		}
	}
	if _, _, err := grpcEndpoint("https://collector.example/otlp"); err == nil {
		t.Fatal("gRPC endpoint with a path was accepted")
	}
}
