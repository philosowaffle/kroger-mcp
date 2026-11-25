// Package telemetry adds OpenTelemetry traces and metrics to the server. It is
// off until an OTLP endpoint is set, and then exports over OTLP HTTP/protobuf,
// configured by the standard OTEL_* variables:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT          turns traces and metrics on
//	OTEL_EXPORTER_OTLP_TRACES_ENDPOINT   turns traces on
//	OTEL_EXPORTER_OTLP_METRICS_ENDPOINT  turns metrics on
//	OTEL_SDK_DISABLED=true               turns both off
//	OTEL_TRACES_EXPORTER=none            turns traces off
//	OTEL_METRICS_EXPORTER=none           turns metrics off
//	OTEL_SERVICE_NAME                    overrides the service name
//	OTEL_RESOURCE_ATTRIBUTES, OTEL_EXPORTER_OTLP_HEADERS, ...
//
// Instrument adds a span and metrics to every MCP call, including the size of
// each response, so the tools whose output would gain most from caching or a
// summary-first shape stand out. HTTPHandler traces the HTTP requests those
// calls arrive in.
package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName names this package's tracer and meter.
const instrumentationName = "github.com/philosowaffle/go-mcp-template/internal/telemetry"

// Attribute keys. The mcp.method.name, gen_ai.* and error.type keys follow
// the OpenTelemetry MCP semantic conventions; the size keys are this
// package's own.
const (
	keyMethod         = attribute.Key("mcp.method.name")
	keyTool           = attribute.Key("gen_ai.tool.name")
	keyOperation      = attribute.Key("gen_ai.operation.name")
	keyRequestSize    = attribute.Key("mcp.request.size")
	keyResponseSize   = attribute.Key("mcp.response.size")
	keyTokensEstimate = attribute.Key("mcp.response.tokens_estimate")
)

// Metric names.
const (
	metricDuration     = "mcp.server.operation.duration"
	metricResponseSize = "mcp.server.response.size"
)

// bytesPerToken is the rough ratio behind the token estimate. Real token
// counts depend on the model's tokenizer and the text; this only ranks
// responses by cost.
const bytesPerToken = 4

// sizeBuckets spread from a short reply to the SDK's 4 MiB cap.
var sizeBuckets = []float64{256, 1 << 10, 4 << 10, 16 << 10, 64 << 10, 256 << 10, 1 << 20, 4 << 20}

// durationBuckets follow the MCP semantic conventions' advice, in seconds.
var durationBuckets = []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}

// Setup installs the global tracer and meter providers when an OTLP endpoint
// is set, and returns a function that flushes and stops them. With no
// endpoint it installs nothing and the returned function does nothing, so
// Instrument and HTTPHandler cost next to nothing.
func Setup(ctx context.Context, name, version string) (shutdown func(context.Context) error, err error) {
	tracesOn, metricsOn := enabled(os.Getenv)
	if !tracesOn && !metricsOn {
		return func(context.Context) error { return nil }, nil
	}
	if p := protocol(os.Getenv); p != "" && p != "http/protobuf" {
		return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_PROTOCOL=%q is not supported: only http/protobuf is (use the collector's HTTP port, usually 4318)", p)
	}

	// resource.Default carries the SDK's own attributes and reads
	// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES, which win over the
	// name and version given here.
	res, err := resource.Merge(
		resource.NewSchemaless(semconv.ServiceName(name), semconv.ServiceVersion(version)),
		resource.Default(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}

	var shutdowns []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdowns {
			errs = append(errs, fn(ctx))
		}
		return errors.Join(errs...)
	}

	if tracesOn {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("trace exporter: %w", err)
		}
		tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
		shutdowns = append(shutdowns, tp.Shutdown)
	}
	if metricsOn {
		exp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			_ = shutdown(ctx)
			return nil, fmt.Errorf("metric exporter: %w", err)
		}
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)), sdkmetric.WithResource(res))
		otel.SetMeterProvider(mp)
		shutdowns = append(shutdowns, mp.Shutdown)
	}

	slog.Info("telemetry enabled", "traces", tracesOn, "metrics", metricsOn)
	return shutdown, nil
}

// enabled reports which signals the environment turns on.
func enabled(getenv func(string) string) (traces, metrics bool) {
	if off, _ := strconv.ParseBool(getenv("OTEL_SDK_DISABLED")); off {
		return false, false
	}
	all := getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
	traces = (all || getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "") &&
		!strings.EqualFold(getenv("OTEL_TRACES_EXPORTER"), "none")
	metrics = (all || getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "") &&
		!strings.EqualFold(getenv("OTEL_METRICS_EXPORTER"), "none")
	return traces, metrics
}

// protocol returns the OTLP protocol the environment asks for, if any.
func protocol(getenv func(string) string) string {
	for _, k := range []string{"OTEL_EXPORTER_OTLP_PROTOCOL", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"} {
		if p := strings.TrimSpace(getenv(k)); p != "" {
			return strings.ToLower(p)
		}
	}
	return ""
}

// HTTPHandler traces each HTTP request to next, continuing any trace the
// caller propagated in a traceparent header, so each MCP call's span sits
// under the request it arrived in. Wrap only the MCP endpoint, so health
// probes stay out of the traces.
func HTTPHandler(next http.Handler, operation string) http.Handler {
	return otelhttp.NewHandler(next, operation)
}

// Instrument adds a span and metrics to every MCP call the server handles,
// and logs each call's size and duration at DEBUG. It reads the global
// providers when each call starts, so it can be added before Setup runs and
// costs next to nothing while telemetry and debug logging are off.
func Instrument(s *mcp.Server) {
	s.AddReceivingMiddleware(middleware)
}

func middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		tracer := otel.GetTracerProvider().Tracer(instrumentationName)
		ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		// Measuring sizes means marshaling the response once more, so do it
		// only when something will see the numbers.
		measure := span.IsRecording() || slog.Default().Enabled(ctx, slog.LevelDebug) || metricsOn()

		attrs := []attribute.KeyValue{keyMethod.String(method)}
		var reqSize int
		if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
			attrs = append(attrs, keyTool.String(p.Name), keyOperation.String("execute_tool"))
			span.SetName(method + " " + p.Name)
			reqSize = len(p.Arguments)
		}
		span.SetAttributes(attrs...)

		start := time.Now()
		res, err := next(ctx, method, req)
		dur := time.Since(start)

		errType := ""
		switch {
		case err != nil:
			errType = "_OTHER"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		case isToolError(res):
			errType = "tool_error"
			span.SetStatus(codes.Error, "tool returned an error")
		}
		if errType != "" {
			attrs = append(attrs, semconv.ErrorTypeKey.String(errType))
			span.SetAttributes(semconv.ErrorTypeKey.String(errType))
		}

		if !measure {
			return res, err
		}
		size := responseSize(res)
		tokens := (size + bytesPerToken - 1) / bytesPerToken
		span.SetAttributes(
			keyRequestSize.Int(reqSize),
			keyResponseSize.Int(size),
			keyTokensEstimate.Int(tokens),
		)
		record(ctx, dur, size, attrs)
		slog.DebugContext(ctx, "mcp call", "method", method, "tool", toolName(req),
			"response_bytes", size, "tokens_estimate", tokens, "dur", dur, "error", errType)
		return res, err
	}
}

// metricsOn reports whether a real meter provider is installed. The global
// default is a delegating no-op until one is.
func metricsOn() bool {
	_, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	return ok
}

// record adds a call to the duration and response-size histograms.
func record(ctx context.Context, dur time.Duration, size int, attrs []attribute.KeyValue) {
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	set := metric.WithAttributes(attrs...)
	if h, err := meter.Float64Histogram(metricDuration,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of MCP requests handled by the server."),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	); err == nil {
		h.Record(ctx, dur.Seconds(), set)
	}
	if h, err := meter.Int64Histogram(metricResponseSize,
		metric.WithUnit("By"),
		metric.WithDescription("Size of MCP responses as JSON, in bytes."),
		metric.WithExplicitBucketBoundaries(sizeBuckets...),
	); err == nil {
		h.Record(ctx, int64(size), set)
	}
}

// responseSize is the length of res as JSON, the bulk of what reaches the
// client and its context window.
func responseSize(res mcp.Result) int {
	if res == nil {
		return 0
	}
	b, err := json.Marshal(res)
	if err != nil {
		return 0
	}
	return len(b)
}

func isToolError(res mcp.Result) bool {
	r, ok := res.(*mcp.CallToolResult)
	return ok && r != nil && r.IsError
}

func toolName(req mcp.Request) string {
	if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
		return p.Name
	}
	return ""
}
