package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// otelEnv lists every variable Setup reads, so tests start from a clean slate.
var otelEnv = []string{
	"OTEL_SDK_DISABLED",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	"OTEL_TRACES_EXPORTER",
	"OTEL_METRICS_EXPORTER",
	"OTEL_EXPORTER_OTLP_PROTOCOL",
	"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
	"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range otelEnv {
		t.Setenv(k, "")
	}
}

// restoreGlobals puts the global OTel providers back after a test.
func restoreGlobals(t *testing.T) {
	t.Helper()
	tp, mp, prop := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(prop)
	})
}

// recordAll installs in-memory providers and returns what they collect.
func recordAll(t *testing.T) (*tracetest.SpanRecorder, *sdkmetric.ManualReader) {
	t.Helper()
	restoreGlobals(t)
	spans := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	return spans, reader
}

type echoArgs struct {
	Text string `json:"text"`
}

type echoOut struct {
	Text string `json:"text"`
}

// newServer builds an instrumented server with an echo tool and a tool that
// always fails.
func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, in echoArgs) (*mcp.CallToolResult, echoOut, error) {
		return nil, echoOut(in), nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "fail"}, func(_ context.Context, _ *mcp.CallToolRequest, _ echoArgs) (*mcp.CallToolResult, echoOut, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "nope"}}}, echoOut{}, nil
	})
	Instrument(s)
	return s
}

func connect(t *testing.T, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name, text string) {
	t.Helper()
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{"text": text}}); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func findSpan(t *testing.T, spans *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	var names []string
	for _, s := range spans.Ended() {
		if s.Name() == name {
			return s
		}
		names = append(names, s.Name())
	}
	t.Fatalf("no span %q; got %v", name, names)
	return nil
}

func attrs(s sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	m := map[attribute.Key]attribute.Value{}
	for _, kv := range s.Attributes() {
		m[kv.Key] = kv.Value
	}
	return m
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name                string
		env                 map[string]string
		wantTraces, wantMet bool
	}{
		{"nothing set", nil, false, false},
		{"endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318"}, true, true},
		{"traces only", map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "http://c:4318/v1/traces"}, true, false},
		{"metrics only", map[string]string{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "http://c:4318/v1/metrics"}, false, true},
		{"sdk disabled", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318", "OTEL_SDK_DISABLED": "true"}, false, false},
		{"traces exporter none", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318", "OTEL_TRACES_EXPORTER": "none"}, false, true},
		{"metrics exporter none", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318", "OTEL_METRICS_EXPORTER": "NONE"}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces, metrics := enabled(func(k string) string { return tt.env[k] })
			if traces != tt.wantTraces || metrics != tt.wantMet {
				t.Errorf("enabled() = %v, %v; want %v, %v", traces, metrics, tt.wantTraces, tt.wantMet)
			}
		})
	}
}

func TestSetup_OffByDefault(t *testing.T) {
	clearEnv(t)
	restoreGlobals(t)
	shutdown, err := Setup(context.Background(), "svc", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown: %v", err)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		t.Error("a tracer provider was installed with no endpoint set")
	}
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); ok {
		t.Error("a meter provider was installed with no endpoint set")
	}
}

func TestSetup_RejectsGRPC(t *testing.T) {
	clearEnv(t)
	restoreGlobals(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	_, err := Setup(context.Background(), "svc", "v1")
	if err == nil || !strings.Contains(err.Error(), "http/protobuf") {
		t.Fatalf("Setup() error = %v, want one naming http/protobuf", err)
	}
}

func TestSetup_EnabledWithEndpoint(t *testing.T) {
	clearEnv(t)
	restoreGlobals(t)
	// Nothing listens here; exporters connect lazily, and nothing is exported
	// before shutdown.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	shutdown, err := Setup(context.Background(), "svc", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Error("no tracer provider installed")
	}
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); !ok {
		t.Error("no meter provider installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // don't wait on the unreachable collector
	_ = shutdown(ctx)
}

func TestInstrument_ToolSpanHasSizes(t *testing.T) {
	spans, _ := recordAll(t)
	cs := connect(t, newServer())
	callTool(t, cs, "echo", strings.Repeat("x", 1000))

	s := findSpan(t, spans, "tools/call echo")
	a := attrs(s)
	if got := a[keyMethod].AsString(); got != "tools/call" {
		t.Errorf("%s = %q", keyMethod, got)
	}
	if got := a[keyTool].AsString(); got != "echo" {
		t.Errorf("%s = %q", keyTool, got)
	}
	if got := a[keyOperation].AsString(); got != "execute_tool" {
		t.Errorf("%s = %q", keyOperation, got)
	}
	// The text comes back twice: as text content and as structured content.
	size := a[keyResponseSize].AsInt64()
	if size < 2000 {
		t.Errorf("%s = %d, want at least 2000", keyResponseSize, size)
	}
	if got, want := a[keyTokensEstimate].AsInt64(), (size+3)/4; got != want {
		t.Errorf("%s = %d, want %d", keyTokensEstimate, got, want)
	}
	if got := a[keyRequestSize].AsInt64(); got < 1000 {
		t.Errorf("%s = %d, want at least 1000", keyRequestSize, got)
	}
	if s.Status().Code == codes.Error {
		t.Errorf("status = %v, want no error", s.Status())
	}
}

func TestInstrument_ToolErrorMarksSpan(t *testing.T) {
	spans, _ := recordAll(t)
	cs := connect(t, newServer())
	callTool(t, cs, "fail", "x")

	s := findSpan(t, spans, "tools/call fail")
	if s.Status().Code != codes.Error {
		t.Errorf("status = %v, want error", s.Status())
	}
	if got := attrs(s)["error.type"].AsString(); got != "tool_error" {
		t.Errorf("error.type = %q, want tool_error", got)
	}
}

func TestInstrument_RecordsHistograms(t *testing.T) {
	_, reader := recordAll(t)
	cs := connect(t, newServer())
	callTool(t, cs, "echo", "hello")
	callTool(t, cs, "echo", "hello again")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Histogram[int64]:
				for _, dp := range data.DataPoints {
					if v, _ := dp.Attributes.Value(keyTool); v.AsString() == "echo" {
						found[m.Name] = dp.Count == 2 && dp.Sum > 0
					}
				}
			case metricdata.Histogram[float64]:
				for _, dp := range data.DataPoints {
					if v, _ := dp.Attributes.Value(keyTool); v.AsString() == "echo" {
						found[m.Name] = dp.Count == 2
					}
				}
			}
		}
	}
	for _, name := range []string{metricResponseSize, metricDuration} {
		if !found[name] {
			t.Errorf("no %s data point for echo with 2 calls; got %v", name, found)
		}
	}
}

func TestHTTPHandler_ToolSpanIsChildOfRequest(t *testing.T) {
	spans, _ := recordAll(t)
	srv := newServer()
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true})
	ts := httptest.NewServer(HTTPHandler(h, "mcp"))
	t.Cleanup(ts.Close)

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	callTool(t, cs, "echo", "hello")

	tool := findSpan(t, spans, "tools/call echo")
	parent := tool.Parent()
	if !parent.IsValid() {
		t.Fatal("tool span has no parent")
	}
	// otelhttp ends the request span after the handler returns, which can be
	// after the client has its response, so wait briefly for it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, s := range spans.Ended() {
			if s.SpanContext().SpanID() == parent.SpanID() {
				if s.InstrumentationScope().Name == instrumentationName {
					t.Fatalf("tool span's parent is another MCP span %q, not the HTTP request", s.Name())
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("tool span's parent is not a recorded HTTP span")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestInstrument_LogsSizeAtDebug(t *testing.T) {
	restoreGlobals(t)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cs := connect(t, newServer())
	callTool(t, cs, "echo", "hello")

	out := buf.String()
	if !strings.Contains(out, "mcp call") || !strings.Contains(out, "tool=echo") || !strings.Contains(out, "response_bytes=") {
		t.Errorf("debug log missing the call's size:\n%s", out)
	}
}
