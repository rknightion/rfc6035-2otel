package selfobs

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecorderEmitsExactSignalCatalog(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	recorder, err := New(provider, BuildInfo{
		Version: "v1.2.3", Revision: "abc123", Date: "2026-08-12", GoVersion: "go1.26.5",
	}, []string{"deskie", "extra"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, outcome := range []string{"accepted", "rejected", "malformed"} {
		if err := recorder.RecordDatagram(ctx, outcome); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.RecordReport(ctx, "STANDARD", "VQSessionReport", "deskie"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordParseError(ctx, "unrecognized_dialect"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordDuplicate(ctx, "extra"); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{200, 405, 415, 489} {
		if err := recorder.RecordResponse(ctx, status); err != nil {
			t.Fatal(err)
		}
	}
	for _, signal := range []string{"metrics", "logs"} {
		if err := recorder.RecordExportFailure(ctx, signal); err != nil {
			t.Fatal(err)
		}
	}
	recorder.RecordDedupeCacheChange(ctx, 1)
	if err := recorder.RecordProcessDuration(ctx, "prestandard", time.Second); err != nil {
		t.Fatal(err)
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatal(err)
	}
	metrics := selfobsMetrics(collected)
	if len(metrics) != 9 {
		t.Fatalf("signals = %d, want 9: %#v", len(metrics), metrics)
	}
	assertAllAttributeKeys(t, metrics)
	// The Prometheus compatibility translation appends _ratio to observable
	// gauges carrying an explicit unit of "1". Build info is dimensionless by
	// definition, so leave the SDK unit unset to preserve the frozen series
	// name rfc6035_2otel_build_info.
	assertGauge(t, metrics, "rfc6035_2otel.build_info", "", map[string]string{
		"service.version": "v1.2.3", "vcs.ref.head.revision": "abc123",
		"rfc6035_2otel.build.date": "2026-08-12", "rfc6035_2otel.build.go_version": "go1.26.5",
	}, 1)
	assertSum(t, metrics, "rfc6035_2otel.datagrams", "{datagram}", true, map[string]string{"rfc6035_2otel.datagram.outcome": "accepted"})
	assertSum(t, metrics, "rfc6035_2otel.reports", "{report}", true, map[string]string{"rfc6035.report.dialect": "standard", "rfc6035.report.type": "VQSessionReport", "rfc6035.sender.name": "deskie"})
	assertSum(t, metrics, "rfc6035_2otel.parse_errors", "{error}", true, map[string]string{"error.type": "unrecognized_dialect"})
	assertSum(t, metrics, "rfc6035_2otel.duplicates", "{report}", true, map[string]string{"rfc6035.sender.name": "extra"})
	assertSum(t, metrics, "rfc6035_2otel.responses", "{response}", true, map[string]string{"rfc6035.response.status_code": "200"})
	assertSum(t, metrics, "rfc6035_2otel.export_failures", "{failure}", true, map[string]string{"rfc6035_2otel.signal": "metrics"})
	assertSum(t, metrics, "rfc6035_2otel.dedupe_cache.usage", "{entry}", false, map[string]string{})
	assertHistogram(t, metrics, "rfc6035_2otel.report.process.duration", "s", map[string]string{"rfc6035.report.dialect": "prestandard"})
}

func TestRecorderRejectsMissingAndUnboundedInput(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	if _, err := New(nil, BuildInfo{Version: "v", Revision: "r", Date: "d", GoVersion: "g"}, nil); err == nil {
		t.Error("nil provider accepted")
	}
	if _, err := New(provider, BuildInfo{}, nil); err == nil {
		t.Error("missing build info accepted")
	}
	recorder, err := New(provider, BuildInfo{Version: "v", Revision: "r", Date: "d", GoVersion: "g"}, []string{"deskie"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, call := range []func() error{
		func() error { return recorder.RecordDatagram(ctx, "") },
		func() error { return recorder.RecordDatagram(ctx, "future") },
		func() error { return recorder.RecordReport(ctx, "", "VQSessionReport", "deskie") },
		func() error { return recorder.RecordReport(ctx, "standard", "", "deskie") },
		func() error { return recorder.RecordReport(ctx, "standard", "VQSessionReport", "192.0.2.1") },
		func() error { return recorder.RecordParseError(ctx, "") },
		func() error { return recorder.RecordParseError(ctx, "future") },
		func() error { return recorder.RecordDuplicate(ctx, "") },
		func() error { return recorder.RecordResponse(ctx, 500) },
		func() error { return recorder.RecordExportFailure(ctx, "traces") },
		func() error { return recorder.RecordProcessDuration(ctx, "", time.Second) },
	} {
		if err := call(); err == nil {
			t.Error("invalid input accepted")
		}
	}
}

func selfobsMetrics(resource metricdata.ResourceMetrics) map[string]metricdata.Metrics {
	result := map[string]metricdata.Metrics{}
	for _, scope := range resource.ScopeMetrics {
		for _, metric := range scope.Metrics {
			result[metric.Name] = metric
		}
	}
	return result
}

func assertGauge(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string, want map[string]string, value float64) {
	t.Helper()
	metric := metrics[name]
	if metric.Unit != unit {
		t.Fatalf("%s unit = %q, want %q", name, metric.Unit, unit)
	}
	gauge, ok := metric.Data.(metricdata.Gauge[float64])
	if !ok {
		t.Fatalf("%s kind = %T, want Gauge", name, metric.Data)
	}
	if len(gauge.DataPoints) != 1 || gauge.DataPoints[0].Value != value {
		t.Fatalf("%s datapoints = %#v", name, gauge.DataPoints)
	}
	assertAttrs(t, name, gauge.DataPoints[0].Attributes, want)
}

func assertSum(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string, monotonic bool, want map[string]string) {
	t.Helper()
	metric := metrics[name]
	if metric.Unit != unit {
		t.Fatalf("%s unit = %q, want %q", name, metric.Unit, unit)
	}
	sum, ok := metric.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%s kind = %T, want Sum", name, metric.Data)
	}
	if sum.IsMonotonic != monotonic {
		t.Fatalf("%s monotonic = %t, want %t", name, sum.IsMonotonic, monotonic)
	}
	if len(sum.DataPoints) == 0 {
		t.Fatalf("%s has no points", name)
	}
	for _, point := range sum.DataPoints {
		if attrsMatch(point.Attributes, want) {
			assertAttrs(t, name, point.Attributes, want)
			return
		}
	}
	t.Fatalf("%s has no matching attributes %#v", name, want)
}

func assertHistogram(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string, want map[string]string) {
	t.Helper()
	metric := metrics[name]
	if metric.Unit != unit {
		t.Fatalf("%s unit = %q, want %q", name, metric.Unit, unit)
	}
	histogram, ok := metric.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%s kind = %T, want Histogram", name, metric.Data)
	}
	if len(histogram.DataPoints) != 1 {
		t.Fatalf("%s points = %d", name, len(histogram.DataPoints))
	}
	assertAttrs(t, name, histogram.DataPoints[0].Attributes, want)
}

func assertAttrs(t *testing.T, name string, got attribute.Set, want map[string]string) {
	t.Helper()
	attrs := map[string]string{}
	for _, kv := range got.ToSlice() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if len(attrs) != len(want) {
		t.Fatalf("%s attrs = %#v, want %#v", name, attrs, want)
	}
	for key, value := range want {
		if attrs[key] != value {
			t.Errorf("%s %s = %q, want %q", name, key, attrs[key], value)
		}
	}
}

func attrsMatch(got attribute.Set, want map[string]string) bool {
	attrs := map[string]string{}
	for _, kv := range got.ToSlice() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	for key, value := range want {
		if attrs[key] != value {
			return false
		}
	}
	return true
}

func assertAllAttributeKeys(t *testing.T, metrics map[string]metricdata.Metrics) {
	t.Helper()
	want := map[string]map[string]bool{
		"rfc6035_2otel.build_info":              {"service.version": true, "vcs.ref.head.revision": true, "rfc6035_2otel.build.date": true, "rfc6035_2otel.build.go_version": true},
		"rfc6035_2otel.datagrams":               {"rfc6035_2otel.datagram.outcome": true},
		"rfc6035_2otel.reports":                 {"rfc6035.report.dialect": true, "rfc6035.report.type": true, "rfc6035.sender.name": true},
		"rfc6035_2otel.parse_errors":            {"error.type": true},
		"rfc6035_2otel.duplicates":              {"rfc6035.sender.name": true},
		"rfc6035_2otel.responses":               {"rfc6035.response.status_code": true},
		"rfc6035_2otel.export_failures":         {"rfc6035_2otel.signal": true},
		"rfc6035_2otel.dedupe_cache.usage":      {},
		"rfc6035_2otel.report.process.duration": {"rfc6035.report.dialect": true},
	}
	for name, metric := range metrics {
		var points []attribute.Set
		switch data := metric.Data.(type) {
		case metricdata.Gauge[float64]:
			for _, point := range data.DataPoints {
				points = append(points, point.Attributes)
			}
		case metricdata.Sum[int64]:
			for _, point := range data.DataPoints {
				points = append(points, point.Attributes)
			}
		case metricdata.Histogram[float64]:
			for _, point := range data.DataPoints {
				points = append(points, point.Attributes)
			}
		default:
			t.Fatalf("unexpected signal kind for %s: %T", name, metric.Data)
		}
		for _, point := range points {
			attrs := point.ToSlice()
			if len(attrs) != len(want[name]) {
				t.Fatalf("%s keys = %#v", name, attrs)
			}
			for _, kv := range attrs {
				if !want[name][string(kv.Key)] {
					t.Fatalf("%s unexpected attribute key %q", name, kv.Key)
				}
			}
		}
	}
}
