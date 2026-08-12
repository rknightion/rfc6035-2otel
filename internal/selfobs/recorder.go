package selfobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationName = "github.com/rknightion/rfc6035-2otel/internal/selfobs"

// BuildInfo is immutable build metadata emitted on build_info.
type BuildInfo struct {
	Version   string
	Revision  string
	Date      string
	GoVersion string
}

// Recorder is the narrow self-observability boundary used by SIP handling and
// report export. Methods reject missing or out-of-registry dimensions rather
// than silently creating a new time series.
type Recorder interface {
	RecordDatagram(context.Context, string) error
	RecordReport(context.Context, string, string, string) error
	RecordParseError(context.Context, string) error
	RecordDuplicate(context.Context, string) error
	RecordResponse(context.Context, int) error
	RecordExportFailure(context.Context, string) error
	RecordDedupeCacheChange(context.Context, int64)
	RecordProcessDuration(context.Context, string, time.Duration) error
}

type recorder struct {
	senders         map[string]string
	datagrams       metric.Int64Counter
	reports         metric.Int64Counter
	parseErrors     metric.Int64Counter
	duplicates      metric.Int64Counter
	responses       metric.Int64Counter
	exportFailures  metric.Int64Counter
	dedupeUsage     metric.Int64UpDownCounter
	processDuration metric.Float64Histogram
}

// New constructs the fixed signal catalog using a caller-owned meter provider.
// senderNames is the complete finite sender-name registry for this process.
//
// SIP should call RecordDatagram, RecordDuplicate, RecordResponse, and
// RecordDedupeCacheChange. The report-processing path should call
// RecordParseError, RecordReport, and RecordProcessDuration. otelexport should
// call RecordExportFailure when its metrics or logs path fails.
func New(meterProvider metric.MeterProvider, build BuildInfo, senderNames []string) (Recorder, error) {
	if meterProvider == nil {
		return nil, errors.New("selfobs: nil meter provider")
	}
	build, err := validateBuildInfo(build)
	if err != nil {
		return nil, err
	}
	senders, err := senderRegistry(senderNames)
	if err != nil {
		return nil, err
	}
	meter := meterProvider.Meter(instrumentationName)
	buildInfo, err := meter.Float64ObservableGauge("rfc6035_2otel.build_info", metric.WithUnit("1"))
	if err != nil {
		return nil, err
	}
	if _, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveFloat64(buildInfo, 1, metric.WithAttributes(
			attribute.String("service.version", build.Version),
			attribute.String("vcs.ref.head.revision", build.Revision),
			attribute.String("rfc6035_2otel.build.date", build.Date),
			attribute.String("rfc6035_2otel.build.go_version", build.GoVersion),
		))
		return nil
	}, buildInfo); err != nil {
		return nil, err
	}
	newCounter := func(name, unit string) (metric.Int64Counter, error) {
		return meter.Int64Counter(name, metric.WithUnit(unit))
	}
	datagrams, err := newCounter("rfc6035_2otel.datagrams", "{datagram}")
	if err != nil {
		return nil, err
	}
	reports, err := newCounter("rfc6035_2otel.reports", "{report}")
	if err != nil {
		return nil, err
	}
	parseErrors, err := newCounter("rfc6035_2otel.parse_errors", "{error}")
	if err != nil {
		return nil, err
	}
	duplicates, err := newCounter("rfc6035_2otel.duplicates", "{report}")
	if err != nil {
		return nil, err
	}
	responses, err := newCounter("rfc6035_2otel.responses", "{response}")
	if err != nil {
		return nil, err
	}
	exportFailures, err := newCounter("rfc6035_2otel.export_failures", "{failure}")
	if err != nil {
		return nil, err
	}
	dedupeUsage, err := meter.Int64UpDownCounter("rfc6035_2otel.dedupe_cache.usage", metric.WithUnit("{entry}"))
	if err != nil {
		return nil, err
	}
	processDuration, err := meter.Float64Histogram("rfc6035_2otel.report.process.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	return &recorder{senders, datagrams, reports, parseErrors, duplicates, responses, exportFailures, dedupeUsage, processDuration}, nil
}

func (r *recorder) RecordDatagram(ctx context.Context, outcome string) error {
	outcome, err := closed("datagram outcome", outcome, "accepted", "rejected", "malformed")
	if err != nil {
		return err
	}
	r.datagrams.Add(ctx, 1, metric.WithAttributes(attribute.String("rfc6035_2otel.datagram.outcome", outcome)))
	return nil
}
func (r *recorder) RecordReport(ctx context.Context, dialect, reportType, sender string) error {
	dialect, err := closed("report dialect", dialect, "standard", "prestandard")
	if err != nil {
		return err
	}
	reportType, err = canonical("report type", reportType, "VQSessionReport", "VQIntervalReport", "VQAlertReport")
	if err != nil {
		return err
	}
	sender, err = r.sender(sender)
	if err != nil {
		return err
	}
	r.reports.Add(ctx, 1, metric.WithAttributes(attribute.String("rfc6035.report.dialect", dialect), attribute.String("rfc6035.report.type", reportType), attribute.String("rfc6035.sender.name", sender)))
	return nil
}
func (r *recorder) RecordParseError(ctx context.Context, errorType string) error {
	errorType, err := closed("error type", errorType, "invalid_input", "malformed_datagram", "malformed_sip", "unrecognized_dialect", "invalid_value", "export_failed")
	if err != nil {
		return err
	}
	r.parseErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("error.type", errorType)))
	return nil
}
func (r *recorder) RecordDuplicate(ctx context.Context, sender string) error {
	sender, err := r.sender(sender)
	if err != nil {
		return err
	}
	r.duplicates.Add(ctx, 1, metric.WithAttributes(attribute.String("rfc6035.sender.name", sender)))
	return nil
}
func (r *recorder) RecordResponse(ctx context.Context, status int) error {
	if status != 200 && status != 405 && status != 415 && status != 489 {
		return fmt.Errorf("selfobs: unsupported response status %d", status)
	}
	r.responses.Add(ctx, 1, metric.WithAttributes(attribute.String("rfc6035.response.status_code", fmt.Sprintf("%d", status))))
	return nil
}
func (r *recorder) RecordExportFailure(ctx context.Context, signal string) error {
	signal, err := closed("signal", signal, "metrics", "logs")
	if err != nil {
		return err
	}
	r.exportFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("rfc6035_2otel.signal", signal)))
	return nil
}
func (r *recorder) RecordDedupeCacheChange(ctx context.Context, delta int64) {
	r.dedupeUsage.Add(ctx, delta)
}
func (r *recorder) RecordProcessDuration(ctx context.Context, dialect string, duration time.Duration) error {
	dialect, err := closed("report dialect", dialect, "standard", "prestandard")
	if err != nil {
		return err
	}
	if duration < 0 {
		return errors.New("selfobs: process duration must not be negative")
	}
	r.processDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("rfc6035.report.dialect", dialect)))
	return nil
}

func validateBuildInfo(build BuildInfo) (BuildInfo, error) {
	build.Version, build.Revision = strings.TrimSpace(build.Version), strings.TrimSpace(build.Revision)
	build.Date, build.GoVersion = strings.TrimSpace(build.Date), strings.TrimSpace(build.GoVersion)
	if build.Version == "" || build.Revision == "" || build.Date == "" || build.GoVersion == "" {
		return BuildInfo{}, errors.New("selfobs: complete build info is required")
	}
	return build, nil
}
func senderRegistry(names []string) (map[string]string, error) {
	result := make(map[string]string, len(names))
	for _, name := range names {
		canonical := strings.TrimSpace(name)
		if canonical == "" {
			return nil, errors.New("selfobs: sender name is required")
		}
		key := strings.ToLower(canonical)
		if _, duplicate := result[key]; duplicate {
			return nil, fmt.Errorf("selfobs: duplicate sender name %q", canonical)
		}
		result[key] = canonical
	}
	return result, nil
}
func (r *recorder) sender(value string) (string, error) {
	canonical, ok := r.senders[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		return "", fmt.Errorf("selfobs: unregistered sender %q", value)
	}
	return canonical, nil
}
func closed(kind, value string, allowed ...string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", fmt.Errorf("selfobs: unsupported %s %q", kind, value)
}
func canonical(kind, value string, allowed ...string) (string, error) {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if strings.EqualFold(value, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("selfobs: unsupported %s %q", kind, value)
}
