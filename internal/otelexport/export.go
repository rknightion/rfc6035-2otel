// Package otelexport exports RFC 6035 voice-quality reports as OpenTelemetry
// metrics and logs.
//
// The OpenTelemetry semantic-convention registry has no voice-quality domain,
// so this package uses the documented rfc6035. prefix. Metrics carry only bounded
// dimensions: parser-owned closed-enum report dialect and type, direction, and
// jitter kind. Call IDs, network addresses, SIP identities, parsed field
// values, and other unbounded data are deliberately log-only. A completed call
// is a point observation, so each metric is a synchronous histogram; this
// retains distributions without pretending reports are counters. No traces are
// emitted: a VQ report is not a distributed-trace operation.
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
	SourcePort    int
	RawReport     string
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
	logger     log.Logger
	senderName func(string) string

	mosLQ, mosCQ, rFactorLQ, rFactorCQ  metric.Float64Histogram
	packetLoss, discardRate             metric.Float64Histogram
	roundTripDelay, oneWayDelay, jitter metric.Float64Histogram
}

// New creates an exporter using caller-owned providers. This makes resource
// configuration and SDK lifecycle the application's responsibility and permits
// in-memory SDK providers in tests.
func New(meterProvider metric.MeterProvider, loggerProvider log.LoggerProvider, senderNames ...func(string) string) (*Exporter, error) {
	if meterProvider == nil {
		return nil, errors.New("otelexport: nil meter provider")
	}
	if loggerProvider == nil {
		return nil, errors.New("otelexport: nil logger provider")
	}
	meter := meterProvider.Meter(instrumentationName)
	newHistogram := func(name, unit string, boundaries []float64) (metric.Float64Histogram, error) {
		return meter.Float64Histogram(name, metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(boundaries...))
	}
	mosBounds := []float64{1, 1.5, 2, 2.5, 3, 3.5, 3.8, 4, 4.2, 4.4, 5}
	rFactorBounds := []float64{0, 20, 40, 50, 60, 70, 80, 90, 94, 100}
	lossBounds := []float64{0, 0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1}
	delayBounds := []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1}

	mosLQ, err := newHistogram("rfc6035.call.mos_lq", "1", mosBounds)
	if err != nil {
		return nil, err
	}
	mosCQ, err := newHistogram("rfc6035.call.mos_cq", "1", mosBounds)
	if err != nil {
		return nil, err
	}
	rFactorLQ, err := newHistogram("rfc6035.call.r_factor_lq", "1", rFactorBounds)
	if err != nil {
		return nil, err
	}
	rFactorCQ, err := newHistogram("rfc6035.call.r_factor_cq", "1", rFactorBounds)
	if err != nil {
		return nil, err
	}
	packetLoss, err := newHistogram("rfc6035.call.packet_loss", "1", lossBounds)
	if err != nil {
		return nil, err
	}
	discardRate, err := newHistogram("rfc6035.call.discard_rate", "1", lossBounds)
	if err != nil {
		return nil, err
	}
	roundTripDelay, err := newHistogram("rfc6035.call.round_trip_delay", "s", delayBounds)
	if err != nil {
		return nil, err
	}
	oneWayDelay, err := newHistogram("rfc6035.call.one_way_delay", "s", delayBounds)
	if err != nil {
		return nil, err
	}
	jitter, err := newHistogram("rfc6035.call.jitter", "s", delayBounds)
	if err != nil {
		return nil, err
	}

	senderName := func(string) string { return "unknown" }
	if len(senderNames) > 0 && senderNames[0] != nil {
		senderName = senderNames[0]
	}
	return &Exporter{
		logger: loggerProvider.Logger(instrumentationName), senderName: senderName,
		mosLQ: mosLQ, mosCQ: mosCQ, rFactorLQ: rFactorLQ, rFactorCQ: rFactorCQ,
		packetLoss: packetLoss, discardRate: discardRate,
		roundTripDelay: roundTripDelay, oneWayDelay: oneWayDelay, jitter: jitter,
	}, nil
}

// Export records all present metric observations and exactly one native OTLP
// log record for report. It does not create spans.
func (e *Exporter) Export(ctx context.Context, report Report) {
	base := []attribute.KeyValue{
		attribute.String("rfc6035.report.dialect", boundedDialect(report.Dialect)),
		attribute.String("rfc6035.report.type", boundedReportType(report.ReportType)),
		attribute.String("rfc6035.sender.name", e.boundedSenderName(report.SourceAddress)),
	}
	e.recordDirection(ctx, e.mosLQ, report.LocalMOSLQ, base, "local", 1)
	e.recordDirection(ctx, e.mosLQ, report.RemoteMOSLQ, base, "remote", 1)
	e.recordDirection(ctx, e.mosCQ, report.LocalMOSCQ, base, "local", 1)
	e.recordDirection(ctx, e.mosCQ, report.RemoteMOSCQ, base, "remote", 1)
	e.recordDirection(ctx, e.rFactorLQ, report.LocalRFactorLQ, base, "local", 1)
	e.recordDirection(ctx, e.rFactorLQ, report.RemoteRFactorLQ, base, "remote", 1)
	e.recordDirection(ctx, e.rFactorCQ, report.LocalRFactorCQ, base, "local", 1)
	e.recordDirection(ctx, e.rFactorCQ, report.RemoteRFactorCQ, base, "remote", 1)
	e.recordDirection(ctx, e.packetLoss, report.LocalPacketLoss, base, "local", 0.01)
	e.recordDirection(ctx, e.packetLoss, report.RemotePacketLoss, base, "remote", 0.01)
	e.recordDirection(ctx, e.discardRate, report.LocalDiscardRate, base, "local", 0.01)
	e.recordDirection(ctx, e.discardRate, report.RemoteDiscardRate, base, "remote", 0.01)
	e.recordDirection(ctx, e.roundTripDelay, report.LocalRTD, base, "local", 0.001)
	e.recordDirection(ctx, e.roundTripDelay, report.RemoteRTD, base, "remote", 0.001)
	e.recordDirection(ctx, e.oneWayDelay, report.LocalOneWayDelay, base, "local", 0.001)
	e.recordDirection(ctx, e.oneWayDelay, report.RemoteOneWayDelay, base, "remote", 0.001)
	e.recordJitter(ctx, report.LocalIAJ, base, "local", "interarrival")
	e.recordJitter(ctx, report.RemoteIAJ, base, "remote", "interarrival")
	e.recordJitter(ctx, report.LocalMAJ, base, "local", "mean_absolute")
	e.recordJitter(ctx, report.RemoteMAJ, base, "remote", "mean_absolute")

	record := log.Record{}
	record.SetEventName("rfc6035.report.received")
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(attribute.StringValue(report.RawReport))
	record.AddAttributes(e.logAttributes(report)...)
	e.logger.Emit(ctx, record)
}

func (e *Exporter) recordDirection(ctx context.Context, histogram metric.Float64Histogram, value *float64, base []attribute.KeyValue, side string, scale float64) {
	if value == nil {
		return
	}
	converted := *value * scale
	attrs := append(append([]attribute.KeyValue{}, base...), attribute.String("rfc6035.report.side", side))
	histogram.Record(ctx, converted, metric.WithAttributes(attrs...))
}

func (e *Exporter) recordJitter(ctx context.Context, value *float64, base []attribute.KeyValue, direction, kind string) {
	if value == nil {
		return
	}
	attrs := append(append([]attribute.KeyValue{}, base...), attribute.String("rfc6035.report.side", direction), attribute.String("rfc6035.jitter.kind", kind))
	e.jitter.Record(ctx, *value/1000, metric.WithAttributes(attrs...))
}

func (e *Exporter) logAttributes(report Report) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("rfc6035.report.call_id", report.CallID),
		attribute.String("rfc6035.report.dialect", boundedDialect(report.Dialect)),
		attribute.String("rfc6035.report.type", boundedReportType(report.ReportType)),
		attribute.String("rfc6035.sender.name", e.boundedSenderName(report.SourceAddress)),
		attribute.String("client.address", report.SourceAddress),
		attribute.Int("client.port", report.SourcePort),
		attribute.String("network.transport", "udp"),
		attribute.String("network.protocol.name", "sip"),
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
		prefix := "rfc6035.field." + normalized
		attrs = append(attrs, attribute.String(prefix, report.Fields[key]))
		if key != normalized {
			attrs = append(attrs, attribute.String(prefix+".original_key", key))
		}
	}
	return attrs
}

func (e *Exporter) boundedSenderName(address string) string {
	name := strings.TrimSpace(e.senderName(address))
	if name == "" {
		return "unknown"
	}
	return name
}

func boundedDialect(value string) string {
	// The parser owns the closed dialect enum. Do not duplicate its cases here:
	// a measured third dialect must remain a one-file parser change.
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	return value
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
