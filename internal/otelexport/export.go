// Package otelexport exports RFC 6035 voice-quality reports as OpenTelemetry
// metrics and logs.
//
// The OpenTelemetry semantic-convention registry has no voice-quality domain,
// so this package uses the documented vq. prefix. Metrics carry only bounded
// dimensions: report dialect and type, direction, and jitter kind. Call IDs,
// network addresses, SIP identities, parsed field values, and other unbounded
// data are deliberately log-only. A completed call is a point observation, so
// each metric is a synchronous histogram; this retains distributions without
// pretending reports are counters. No traces are emitted: a VQ report is not a
// distributed-trace operation.
package otelexport

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/rknightion/rfc6035-2otel/internal/otelexport"

// Report is the exporter-owned, parser-neutral view of a voice-quality report.
// Nil metric pointers mean that the report did not contain that observation and
// therefore must not produce a zero-valued metric.
type Report struct {
	Dialect       string
	ReportType    string
	CallID        string
	SourceAddress string
	Fields        map[string]string

	LocalMOSLQ  *float64
	RemoteMOSLQ *float64
	LocalMOSCQ  *float64
	RemoteMOSCQ *float64

	LocalRFactorLQ  *float64
	RemoteRFactorLQ *float64
	LocalRFactorCQ  *float64
	RemoteRFactorCQ *float64

	LocalPacketLoss   *float64
	RemotePacketLoss  *float64
	LocalDiscardRate  *float64
	RemoteDiscardRate *float64
	LocalRTD          *float64
	RemoteRTD         *float64
	LocalOneWayDelay  *float64
	RemoteOneWayDelay *float64
	LocalIAJ          *float64
	RemoteIAJ         *float64
	LocalMAJ          *float64
	RemoteMAJ         *float64
}

// Exporter owns the synchronous instruments and native OTLP logger used for
// report observations. It is safe for concurrent use.
type Exporter struct {
	logger log.Logger

	mosLQ, mosCQ, rFactorLQ, rFactorCQ  metric.Float64Histogram
	packetLoss, discardRate             metric.Float64Histogram
	roundTripDelay, oneWayDelay, jitter metric.Float64Histogram
}

// New creates an exporter using caller-owned providers. This makes resource
// configuration and SDK lifecycle the application's responsibility and permits
// in-memory SDK providers in tests.
func New(meterProvider metric.MeterProvider, loggerProvider log.LoggerProvider) (*Exporter, error) {
	if meterProvider == nil {
		return nil, errors.New("otelexport: nil meter provider")
	}
	if loggerProvider == nil {
		return nil, errors.New("otelexport: nil logger provider")
	}
	meter := meterProvider.Meter(instrumentationName)
	newHistogram := func(name, unit string) (metric.Float64Histogram, error) {
		return meter.Float64Histogram(name, metric.WithUnit(unit))
	}

	mosLQ, err := newHistogram("vq.call.mos_lq", "1")
	if err != nil {
		return nil, err
	}
	mosCQ, err := newHistogram("vq.call.mos_cq", "1")
	if err != nil {
		return nil, err
	}
	rFactorLQ, err := newHistogram("vq.call.r_factor_lq", "1")
	if err != nil {
		return nil, err
	}
	rFactorCQ, err := newHistogram("vq.call.r_factor_cq", "1")
	if err != nil {
		return nil, err
	}
	packetLoss, err := newHistogram("vq.call.packet_loss", "%")
	if err != nil {
		return nil, err
	}
	discardRate, err := newHistogram("vq.call.discard_rate", "%")
	if err != nil {
		return nil, err
	}
	roundTripDelay, err := newHistogram("vq.call.round_trip_delay", "ms")
	if err != nil {
		return nil, err
	}
	oneWayDelay, err := newHistogram("vq.call.one_way_delay", "ms")
	if err != nil {
		return nil, err
	}
	jitter, err := newHistogram("vq.call.jitter", "ms")
	if err != nil {
		return nil, err
	}

	return &Exporter{
		logger: loggerProvider.Logger(instrumentationName),
		mosLQ:  mosLQ, mosCQ: mosCQ, rFactorLQ: rFactorLQ, rFactorCQ: rFactorCQ,
		packetLoss: packetLoss, discardRate: discardRate,
		roundTripDelay: roundTripDelay, oneWayDelay: oneWayDelay, jitter: jitter,
	}, nil
}

// Export records all present metric observations and exactly one native OTLP
// log record for report. It does not create spans.
func (e *Exporter) Export(ctx context.Context, report Report) {
	base := []attribute.KeyValue{
		attribute.String("vq.report.dialect", boundedDialect(report.Dialect)),
		attribute.String("vq.report.type", boundedReportType(report.ReportType)),
	}
	e.recordDirection(ctx, e.mosLQ, report.LocalMOSLQ, base, "local")
	e.recordDirection(ctx, e.mosLQ, report.RemoteMOSLQ, base, "remote")
	e.recordDirection(ctx, e.mosCQ, report.LocalMOSCQ, base, "local")
	e.recordDirection(ctx, e.mosCQ, report.RemoteMOSCQ, base, "remote")
	e.recordDirection(ctx, e.rFactorLQ, report.LocalRFactorLQ, base, "local")
	e.recordDirection(ctx, e.rFactorLQ, report.RemoteRFactorLQ, base, "remote")
	e.recordDirection(ctx, e.rFactorCQ, report.LocalRFactorCQ, base, "local")
	e.recordDirection(ctx, e.rFactorCQ, report.RemoteRFactorCQ, base, "remote")
	e.recordDirection(ctx, e.packetLoss, report.LocalPacketLoss, base, "local")
	e.recordDirection(ctx, e.packetLoss, report.RemotePacketLoss, base, "remote")
	e.recordDirection(ctx, e.discardRate, report.LocalDiscardRate, base, "local")
	e.recordDirection(ctx, e.discardRate, report.RemoteDiscardRate, base, "remote")
	e.recordDirection(ctx, e.roundTripDelay, report.LocalRTD, base, "local")
	e.recordDirection(ctx, e.roundTripDelay, report.RemoteRTD, base, "remote")
	e.recordDirection(ctx, e.oneWayDelay, report.LocalOneWayDelay, base, "local")
	e.recordDirection(ctx, e.oneWayDelay, report.RemoteOneWayDelay, base, "remote")
	e.recordJitter(ctx, report.LocalIAJ, base, "local", "IAJ")
	e.recordJitter(ctx, report.RemoteIAJ, base, "remote", "IAJ")
	e.recordJitter(ctx, report.LocalMAJ, base, "local", "MAJ")
	e.recordJitter(ctx, report.RemoteMAJ, base, "remote", "MAJ")

	record := log.Record{}
	record.SetEventName("vq.report.received")
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(attribute.StringValue("voice quality report"))
	record.AddAttributes(e.logAttributes(report)...)
	e.logger.Emit(ctx, record)
}

func (e *Exporter) recordDirection(ctx context.Context, histogram metric.Float64Histogram, value *float64, base []attribute.KeyValue, direction string) {
	if value == nil {
		return
	}
	attrs := append(append([]attribute.KeyValue{}, base...), attribute.String("vq.direction", direction))
	histogram.Record(ctx, *value, metric.WithAttributes(attrs...))
}

func (e *Exporter) recordJitter(ctx context.Context, value *float64, base []attribute.KeyValue, direction, kind string) {
	if value == nil {
		return
	}
	attrs := append(append([]attribute.KeyValue{}, base...), attribute.String("vq.direction", direction), attribute.String("vq.jitter.type", kind))
	e.jitter.Record(ctx, *value, metric.WithAttributes(attrs...))
}

func (e *Exporter) logAttributes(report Report) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("event.name", "vq.report.received"),
		attribute.String("vq.report.call_id", report.CallID),
		attribute.String("vq.report.dialect", boundedDialect(report.Dialect)),
		attribute.String("vq.report.type", boundedReportType(report.ReportType)),
		attribute.String("client.address", report.SourceAddress),
	}
	keys := make([]string, 0, len(report.Fields))
	for key := range report.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	used := make(map[string]bool, len(keys))
	for _, key := range keys {
		if secretField(key) {
			continue
		}
		base := normalizeFieldKey(key)
		normalized := base
		for suffix := "x"; used[normalized]; suffix += "x" {
			normalized = base + "_" + suffix
		}
		used[normalized] = true
		prefix := "vq.field." + normalized
		attrs = append(attrs, attribute.String(prefix, report.Fields[key]), attribute.String(prefix+".original_key", key))
	}
	return attrs
}

func boundedDialect(value string) string {
	switch strings.ToLower(value) {
	case "standard":
		return "standard"
	case "prestandard", "pre-standard":
		return "prestandard"
	default:
		return "unknown"
	}
}

func boundedReportType(value string) string {
	switch value {
	case "VQSessionReport", "VQIntervalReport", "VQAlertReport":
		return value
	default:
		return "unknown"
	}
}

func normalizeFieldKey(key string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(key) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "unnamed"
	}
	return result
}

func secretField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return strings.Contains(normalized, "authorization") || strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "cookie") || strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "token") || normalized == "auth"
}
