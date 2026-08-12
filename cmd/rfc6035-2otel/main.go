// Command rfc6035-2otel receives RFC 6035 SIP PUBLISH reports and exports
// voice-quality observations over OTLP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/rknightion/rfc6035-2otel/internal/config"
	"github.com/rknightion/rfc6035-2otel/internal/otelexport"
	"github.com/rknightion/rfc6035-2otel/internal/sip"
	"github.com/rknightion/rfc6035-2otel/internal/vqreport"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("rfc6035-2otel", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to YAML configuration")
	healthAddress := flags.String("healthcheck", "", "send a SIP liveness probe to host:port")
	showVersion := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "%s (commit %s, built %s)\n", version, commit, buildDate)
		return 0
	}
	if *healthAddress != "" {
		if err := healthcheck(ctx, *healthAddress); err != nil {
			fmt.Fprintf(stderr, "healthcheck: %v\n", err)
			return 1
		}
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load configuration: %v\n", err)
		return 1
	}
	logger, err := newLogger(cfg.Log.Level, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "configure logging: %v\n", err)
		return 1
	}
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error("OpenTelemetry SDK error", "error", err)
	}))

	providers, err := newProviders(ctx, *cfg)
	if err != nil {
		logger.Error("build OpenTelemetry providers", "error", err)
		return 1
	}
	exporter, err := otelexport.New(providers.metrics, providers.logs)
	if err != nil {
		logger.Error("build report exporter", "error", err)
		_ = providers.Shutdown(context.Background())
		return 1
	}

	listener, err := sip.New(sip.Config{
		Address:      net.JoinHostPort(cfg.Listen.Address, strconv.Itoa(cfg.Listen.Port)),
		DedupeWindow: cfg.DedupeWindow,
		Handler: func(handlerCtx context.Context, publish sip.Publish) {
			report, parseErr := vqreport.Parse(publish.Body)
			if parseErr != nil {
				logger.Warn("reject voice-quality report", "source", publish.RemoteAddr.String(), "sip_call_id", publish.CallID, "error", parseErr)
				return
			}
			exporter.Export(handlerCtx, exportReport(report, publish))
			logger.Info("export voice-quality report", "source", publish.RemoteAddr.String(), "call_id", report.CallID, "report_type", report.ReportType, "dialect", report.Dialect.String())
		},
	})
	if err != nil {
		logger.Error("configure SIP listener", "error", err)
		_ = providers.Shutdown(context.Background())
		return 1
	}

	logger.Info("rfc6035-2otel started", "version", version, "commit", commit, "listen", net.JoinHostPort(cfg.Listen.Address, strconv.Itoa(cfg.Listen.Port)))
	serveErr := listener.ListenAndServe(ctx)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := providers.Shutdown(shutdownCtx)
	if serveErr != nil {
		logger.Error("SIP listener stopped", "error", serveErr)
	}
	if shutdownErr != nil {
		logger.Error("shutdown OpenTelemetry providers", "error", shutdownErr)
	}
	if serveErr != nil || shutdownErr != nil {
		return 1
	}
	logger.Info("rfc6035-2otel stopped")
	return 0
}

func newLogger(level string, writer io.Writer) (*slog.Logger, error) {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	parsed, ok := levels[level]
	if !ok {
		return nil, fmt.Errorf("unknown log level %q", level)
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: parsed})), nil
}

type providers struct {
	metrics *sdkmetric.MeterProvider
	logs    *sdklog.LoggerProvider
}

func newProviders(ctx context.Context, cfg config.Config) (*providers, error) {
	var metricExporter sdkmetric.Exporter
	var logExporter sdklog.Exporter
	var err error
	switch cfg.OTLP.Protocol {
	case "http":
		metricExporter, err = otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpointURL(otlpHTTPURL(cfg.OTLP.Endpoint, "metrics")),
			otlpmetrichttp.WithHeaders(cfg.OTLP.Headers),
		)
		if err != nil {
			return nil, fmt.Errorf("create OTLP/HTTP metric exporter: %w", err)
		}
		logExporter, err = otlploghttp.New(ctx,
			otlploghttp.WithEndpointURL(otlpHTTPURL(cfg.OTLP.Endpoint, "logs")),
			otlploghttp.WithHeaders(cfg.OTLP.Headers),
		)
		if err != nil {
			return nil, fmt.Errorf("create OTLP/HTTP log exporter: %w", err)
		}
	case "grpc":
		endpoint, insecure, parseErr := grpcEndpoint(cfg.OTLP.Endpoint)
		if parseErr != nil {
			return nil, parseErr
		}
		metricOptions := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithHeaders(cfg.OTLP.Headers),
		}
		logOptions := []otlploggrpc.Option{
			otlploggrpc.WithEndpoint(endpoint),
			otlploggrpc.WithHeaders(cfg.OTLP.Headers),
		}
		if insecure {
			metricOptions = append(metricOptions, otlpmetricgrpc.WithInsecure())
			logOptions = append(logOptions, otlploggrpc.WithInsecure())
		}
		metricExporter, err = otlpmetricgrpc.New(ctx, metricOptions...)
		if err != nil {
			return nil, fmt.Errorf("create OTLP/gRPC metric exporter: %w", err)
		}
		logExporter, err = otlploggrpc.New(ctx, logOptions...)
		if err != nil {
			return nil, fmt.Errorf("create OTLP/gRPC log exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol %q", cfg.OTLP.Protocol)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.Service.Name),
		semconv.ServiceVersion(cfg.Service.Version),
		semconv.ServiceInstanceID(hostname()),
	))
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
	)
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	return &providers{metrics: metricProvider, logs: logProvider}, nil
}

func (p *providers) Shutdown(ctx context.Context) error {
	return errors.Join(p.metrics.Shutdown(ctx), p.logs.Shutdown(ctx))
}

func otlpHTTPURL(base, signal string) string {
	base = strings.TrimRight(base, "/")
	for _, existing := range []string{"metrics", "logs"} {
		base = strings.TrimSuffix(base, "/v1/"+existing)
	}
	return base + "/v1/" + signal
}

func grpcEndpoint(raw string) (endpoint string, insecure bool, err error) {
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("parse OTLP/gRPC endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false, fmt.Errorf("otlp.endpoint scheme must be http or https for gRPC")
	}
	if parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", false, fmt.Errorf("otlp.endpoint for gRPC must contain only a host and optional port")
	}
	return parsed.Host, parsed.Scheme == "http", nil
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "unknown"
	}
	return name
}

func exportReport(report vqreport.Report, publish sip.Publish) otelexport.Report {
	fields := make(map[string]string, len(report.Fields)+2)
	for _, field := range report.Fields {
		key := field.Key
		if _, exists := fields[key]; exists {
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s[%d]", key, n)
				if _, found := fields[candidate]; !found {
					key = candidate
					break
				}
			}
		}
		fields[key] = field.Value
	}
	fields["SIP.Call-ID"] = publish.CallID
	fields["SIP.CSeq"] = publish.CSeq
	return otelexport.Report{
		Dialect:           report.Dialect.String(),
		ReportType:        report.ReportType,
		CallID:            report.CallID,
		SourceAddress:     sourceAddress(publish.RemoteAddr),
		Fields:            fields,
		LocalMOSLQ:        report.LocalMetrics.MOSLQ,
		RemoteMOSLQ:       report.RemoteMetrics.MOSLQ,
		LocalMOSCQ:        report.LocalMetrics.MOSCQ,
		RemoteMOSCQ:       report.RemoteMetrics.MOSCQ,
		LocalRFactorLQ:    report.LocalMetrics.RLQ,
		RemoteRFactorLQ:   report.RemoteMetrics.RLQ,
		LocalRFactorCQ:    report.LocalMetrics.RCQ,
		RemoteRFactorCQ:   report.RemoteMetrics.RCQ,
		LocalPacketLoss:   report.LocalMetrics.PacketLoss,
		RemotePacketLoss:  report.RemoteMetrics.PacketLoss,
		LocalDiscardRate:  report.LocalMetrics.DiscardRate,
		RemoteDiscardRate: report.RemoteMetrics.DiscardRate,
		LocalRTD:          report.LocalMetrics.RoundTripDelay,
		RemoteRTD:         report.RemoteMetrics.RoundTripDelay,
		LocalOneWayDelay:  report.LocalMetrics.OneWayDelay,
		RemoteOneWayDelay: report.RemoteMetrics.OneWayDelay,
		LocalIAJ:          report.LocalMetrics.IAJ,
		RemoteIAJ:         report.RemoteMetrics.IAJ,
		LocalMAJ:          report.LocalMetrics.MAJ,
		RemoteMAJ:         report.RemoteMetrics.MAJ,
	}
}

func sourceAddress(address net.Addr) string {
	if udp, ok := address.(*net.UDPAddr); ok {
		return udp.IP.String()
	}
	host, _, err := net.SplitHostPort(address.String())
	if err == nil {
		return host
	}
	return address.String()
}

func healthcheck(ctx context.Context, address string) error {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	request := "OPTIONS sip:" + address + " SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP 127.0.0.1:5061;branch=z9hG4bKhealth;rport\r\n" +
		"From: <sip:health@localhost>;tag=health\r\n" +
		"To: <sip:" + address + ">\r\n" +
		"Call-ID: healthcheck\r\n" +
		"CSeq: 1 OPTIONS\r\nContent-Length: 0\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return err
	}
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(response[:n]), "SIP/2.0 405 ") {
		return fmt.Errorf("unexpected SIP response %q", strings.TrimSpace(string(response[:n])))
	}
	return nil
}
