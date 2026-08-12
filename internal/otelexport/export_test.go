package otelexport

import (
	"context"
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

	exporter, err := New(mp, lp)
	if err != nil {
		t.Fatal(err)
	}
	mos := 4.2
	jitter := 7.0
	packetLoss := 0.5
	exporter.Export(context.Background(), Report{
		Dialect: "standard", ReportType: "VQSessionReport", CallID: "call-123", SourceAddress: "192.0.2.5",
		Fields: map[string]string{
			"X-Vendor": "kept", "X_Vendor": "also-kept", "X_Vendor_X": "third-kept",
			"Authorization": "never-export",
		},
		LocalMOSLQ: &mos, LocalIAJ: &jitter, LocalPacketLoss: &packetLoss,
	})

	var got metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	metrics := flattenMetrics(got)
	assertHistogram(t, metrics, "vq.call.mos_lq", "1", map[string]string{
		"vq.report.dialect": "standard", "vq.report.type": "VQSessionReport", "vq.direction": "local",
	})
	assertHistogram(t, metrics, "vq.call.jitter", "ms", map[string]string{
		"vq.report.dialect": "standard", "vq.report.type": "VQSessionReport", "vq.direction": "local", "vq.jitter.type": "IAJ",
	})
	assertHistogram(t, metrics, "vq.call.packet_loss", "%", map[string]string{
		"vq.report.dialect": "standard", "vq.report.type": "VQSessionReport", "vq.direction": "local",
	})
	if _, ok := metrics["vq.call.mos_cq"]; ok {
		t.Fatal("nil MOS-CQ emitted a metric")
	}
	for _, metric := range metrics {
		for _, point := range metric.Data.(metricdata.Histogram[float64]).DataPoints {
			if point.Attributes.HasValue(attribute.Key("vq.report.call_id")) {
				t.Fatal("call ID appeared on metric")
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
	if record.Body().AsString() != "voice quality report" {
		t.Fatalf("body = %q", record.Body().AsString())
	}
	attrs := recordAttributes(record)
	for key, want := range map[string]string{
		"event.name": "vq.report.received", "vq.report.call_id": "call-123", "client.address": "192.0.2.5",
		"vq.field.x_vendor": "kept", "vq.field.x_vendor_x": "also-kept",
		"vq.field.x_vendor_x_x": "third-kept",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("attribute %s = %q, want %q", key, got, want)
		}
	}
	if _, found := attrs["vq.field.authorization"]; found {
		t.Error("secret field was logged")
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

func assertHistogram(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string, want map[string]string) {
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
	got := attributeSet(histogram.DataPoints[0].Attributes)
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
	record.WalkAttributes(func(kv attribute.KeyValue) bool { result[string(kv.Key)] = kv.Value.AsString(); return true })
	return result
}
