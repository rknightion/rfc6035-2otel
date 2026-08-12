package otelexport

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestExportMetricsAndLog(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	logs := &recordingLogExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs)))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()); _ = lp.Shutdown(context.Background()) })

	exporter, err := New(mp, lp, func(address string) string {
		if address == "10.0.0.139" {
			return "deskie"
		}
		return "unknown"
	})
	if err != nil {
		t.Fatal(err)
	}
	mos := 4.2
	jitter := 250.0
	packetLoss := 2.0
	rFactor := 80.0
	discardRate := 2.0
	delay := 250.0
	exporter.Export(context.Background(), Report{
		Dialect: "standard", ReportType: "VQSessionReport", CallID: "call-123", SourceAddress: "10.0.0.139", SourcePort: 5060,
		RawReport: "VQSessionReport\r\nCall-ID: call-123\r\nIAJ=250\r\nPL=2\r\n",
		Fields: map[string]string{
			"X-Vendor": "kept", "X_Vendor": "also-kept", "X_Vendor_X": "third-kept",
			"Authorization": "never-export",
		},
		LocalMOSLQ: &mos, LocalMOSCQ: &mos, LocalIAJ: &jitter, LocalPacketLoss: &packetLoss,
		LocalRFactorLQ: &rFactor, LocalRFactorCQ: &rFactor, LocalDiscardRate: &discardRate, LocalRTD: &delay, LocalOneWayDelay: &delay,
	})

	var got metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	metrics := flattenMetrics(got)
	assertHistogram(t, metrics, "rfc6035.call.mos_lq", "1", []float64{1, 1.5, 2, 2.5, 3, 3.5, 3.8, 4, 4.2, 4.4, 5}, 4.2, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.mos_cq", "1", []float64{1, 1.5, 2, 2.5, 3, 3.5, 3.8, 4, 4.2, 4.4, 5}, 4.2, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.jitter", "s", []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1}, 0.25, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.jitter.kind": "interarrival", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.packet_loss", "1", []float64{0, 0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1}, 0.02, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.r_factor_lq", "1", []float64{0, 20, 40, 50, 60, 70, 80, 90, 94, 100}, 80, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.r_factor_cq", "1", []float64{0, 20, 40, 50, 60, 70, 80, 90, 94, 100}, 80, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.discard_rate", "1", []float64{0, 0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1}, 0.02, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.round_trip_delay", "s", []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1}, 0.25, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	assertHistogram(t, metrics, "rfc6035.call.one_way_delay", "s", []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1}, 0.25, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "deskie",
	})
	for name, metric := range metrics {
		if strings.Contains(name, "vq"+".") {
			t.Fatalf("legacy metric emitted: %s", name)
		}
		for _, point := range metric.Data.(metricdata.Histogram[float64]).DataPoints {
			if point.Attributes.HasValue(attribute.Key("rfc6035.report.call_id")) {
				t.Fatal("call ID appeared on metric")
			}
			for key := range attributeSet(point.Attributes) {
				if !strings.Contains(key, ".") {
					t.Fatalf("metric emitted unnamespaced attribute %q", key)
				}
			}
		}
	}

	if len(logs.records) != 1 {
		t.Fatalf("log records = %d, want 1", len(logs.records))
	}
	record := logs.records[0]
	if record.Severity() != log.SeverityInfo {
		t.Fatalf("severity = %v, want INFO", record.Severity())
	}
	if record.EventName() != "rfc6035.report.received" {
		t.Fatalf("event name = %q", record.EventName())
	}
	if record.Body().AsString() != "VQSessionReport\r\nCall-ID: call-123\r\nIAJ=250\r\nPL=2\r\n" {
		t.Fatalf("body = %q", record.Body().AsString())
	}
	attrs := recordAttributes(record)
	for key, want := range map[string]string{
		"rfc6035.report.call_id": "call-123", "rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport",
		"rfc6035.sender.name": "deskie", "client.address": "10.0.0.139", "client.port": "5060",
		"network.transport": "udp", "network.protocol.name": "sip",
		"rfc6035.field.x_vendor": "kept", "rfc6035.field.x_vendor.original_key": "X-Vendor",
		"rfc6035.field.x_vendor_x": "also-kept", "rfc6035.field.x_vendor_x.original_key": "X_Vendor",
		"rfc6035.field.x_vendor_x_x": "third-kept", "rfc6035.field.x_vendor_x_x.original_key": "X_Vendor_X",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s = %q, want %q", key, got, want)
		}
	}
	if _, found := attrs["event.name"]; found {
		t.Error("event.name was emitted as an attribute")
	}
	if _, found := attrs["rfc6035.field.authorization"]; found {
		t.Error("secret field was logged")
	}
	for key := range attrs {
		if !strings.Contains(key, ".") {
			t.Fatalf("log emitted unnamespaced attribute %q", key)
		}
	}
}

func TestExportUnknownSenderAndNormalFieldKey(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	logs := &recordingLogExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs)))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()); _ = lp.Shutdown(context.Background()) })
	exporter, err := New(mp, lp, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	mos := 4.0
	exporter.Export(context.Background(), Report{Dialect: "standard", ReportType: "VQSessionReport", SourceAddress: "192.0.2.1", Fields: map[string]string{"normal_key": "retained"}, LocalMOSLQ: &mos})
	var got metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	assertHistogram(t, flattenMetrics(got), "rfc6035.call.mos_lq", "1", []float64{1, 1.5, 2, 2.5, 3, 3.5, 3.8, 4, 4.2, 4.4, 5}, 4, map[string]string{
		"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.report.side": "local", "rfc6035.sender.name": "unknown",
	})
	attrs := recordAttributes(logs.records[0])
	if got := attrs["rfc6035.field.normal_key"]; got != "retained" {
		t.Fatalf("normal key = %q", got)
	}
	if _, found := attrs["rfc6035.field.normal_key.original_key"]; found {
		t.Fatal("normal key unnecessarily retained original key")
	}
}

func TestDialectEnumHasOneOwner(t *testing.T) {
	if got := boundedDialect("Future-Measured-Dialect"); got != "future-measured-dialect" {
		t.Fatalf("boundedDialect = %q; exporter duplicated the parser enum", got)
	}
	if got := boundedDialect("  "); got != "unknown" {
		t.Fatalf("empty dialect = %q", got)
	}
}

type recordingLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}
func (*recordingLogExporter) Shutdown(context.Context) error   { return nil }
func (*recordingLogExporter) ForceFlush(context.Context) error { return nil }

func flattenMetrics(resource metricdata.ResourceMetrics) map[string]metricdata.Metrics {
	result := map[string]metricdata.Metrics{}
	for _, scope := range resource.ScopeMetrics {
		for _, metric := range scope.Metrics {
			result[metric.Name] = metric
		}
	}
	return result
}

func assertHistogram(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string, boundaries []float64, value float64, want map[string]string) {
	t.Helper()
	metric, found := metrics[name]
	if !found {
		t.Fatalf("missing metric %s", name)
	}
	if metric.Unit != unit {
		t.Fatalf("metric %s unit = %q, want %q", name, metric.Unit, unit)
	}
	histogram, ok := metric.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("metric %s data = %T, want Histogram[float64]", name, metric.Data)
	}
	if len(histogram.DataPoints) != 1 {
		t.Fatalf("metric %s points = %d, want 1", name, len(histogram.DataPoints))
	}
	point := histogram.DataPoints[0]
	if point.Sum != value {
		t.Fatalf("metric %s sum = %v, want %v", name, point.Sum, value)
	}
	if fmt.Sprint(point.Bounds) != fmt.Sprint(boundaries) {
		t.Fatalf("metric %s bounds = %v, want %v", name, point.Bounds, boundaries)
	}
	got := attributeSet(point.Attributes)
	if len(got) != len(want) {
		t.Fatalf("metric %s attributes = %#v, want %#v", name, got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("metric %s %s = %q, want %q", name, key, got[key], value)
		}
	}
}

func attributeSet(set attribute.Set) map[string]string {
	result := map[string]string{}
	for _, kv := range set.ToSlice() {
		result[string(kv.Key)] = kv.Value.AsString()
	}
	return result
}
func recordAttributes(record sdklog.Record) map[string]string {
	result := map[string]string{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		result[string(kv.Key)] = fmt.Sprint(kv.Value.AsInterface())
		return true
	})
	return result
}
