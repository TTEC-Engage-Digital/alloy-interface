package alloyinterface

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type AlloyClient struct {
	Tracer         trace.Tracer
	Meter          metric.Meter
	cfg            Config
	traceShutdown  func(context.Context) error
	metricShutdown func(context.Context) error
}

func NewAlloyClient(ctx context.Context) (*AlloyClient, error) {
	cfg := LoadConfig()

	tracer, closeFn, err := initTracer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &AlloyClient{
		Tracer:        tracer,
		cfg:           cfg,
		traceShutdown: closeFn,
	}, nil
}

func (ac *AlloyClient) StartTrace(ctx context.Context, name string) (context.Context, trace.Span, error) {
	if ac.Tracer == nil {
		return nil, nil, errors.New("tracer not initialized")
	}
	ctx, span := ac.Tracer.Start(ctx, name)
	return ctx, span, nil
}

func (ac *AlloyClient) AddSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) error {
	if ac.Tracer == nil {
		return errors.New("tracer not initialized")
	}
	_, span := ac.Tracer.Start(ctx, name)
	span.SetAttributes(attrs...)
	span.End()
	return nil
}

// func (ac *AlloyClient) AddLog(ctx context.Context, title string, logMsgs string) error {
// 	_, span, err := ac.StartTrace(ctx, "log")
// 	if err != nil {
// 		return fmt.Errorf("failed to start tracing: %v", err)
// 	}

// 	span.SetAttributes(attribute.String(title, logMsgs))
// 	span.End()
// 	return nil
// }

func (ac *AlloyClient) AddLog(ctx context.Context, level string, msg string, attrs ...any) (*http.Response, error) {
	attrMap := make(map[string]interface{})
	for i := 0; i < len(attrs); i += 2 {
		key, found := attrs[i].(string)
		if !found {
			continue
		}
		attrMap[key] = attrs[i+1]
	}

	logRecord := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"severity_text": level,
		"body":          msg,
		"attributes":    attrMap,
	}

	jsonBytes, err := json.Marshal(logRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log record: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ac.cfg.Endpoint+"/v1/logs", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return resp, fmt.Errorf("failed to send log record, status code: %d", resp.StatusCode)
	}

	return resp, nil
}

func (ac *AlloyClient) Shutdown(ctx context.Context) error {
	var shutdownErrs []error
	if ac.traceShutdown != nil {
		if err := ac.traceShutdown(ctx); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("failed to shutdown tracer: %v", err))
		}
	}

	if len(shutdownErrs) > 0 {
		return fmt.Errorf("shutdown errors: %v", shutdownErrs)
	}
	return nil
}

func initTracer(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	httpOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(),
	}
	exporter, err = otlptracehttp.New(ctx, httpOpts...)
	if err != nil {
		return nil, nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return otel.Tracer(cfg.TracerName), tp.Shutdown, nil
}

// func initMetrics(ctx context.Context, cfg Config) (metric.Meter, func(context.Context) error, error) {
// 	metricExp, err := otlptracehttp.New(ctx,
// 		otlptracehttp.WithEndpoint(cfg.MetricsEndpoint),
// 		otlptracehttp.WithInsecure(),
// 	)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	mp := sdkmetric.NewMeterProvider(
// 		sdkmetric.WithReader(metricExp),
// }

func newLogger() (*slog.Logger, *os.File, error) {
	today := time.Now().Format("2006-01-02")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	logDir := filepath.Join(homeDir, "logs", "alloy-interface")
	logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", today))

	err = os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating log directory:", err)
		return nil, nil, err
	}

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return nil, nil, err
	}

	handler := slog.NewJSONHandler(file, nil)
	return slog.New(handler), file, nil
}

func slogLevelToString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
