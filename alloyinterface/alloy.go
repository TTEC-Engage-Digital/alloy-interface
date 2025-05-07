package alloyinterface

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	Logger         *slog.Logger
	Meter          metric.Meter
	cfg            Config
	traceShutdown  func(context.Context) error
	metricShutdown func(context.Context) error
}

func NewAlloyClient(ctx context.Context) (*AlloyClient, error) {
	cfg := LoadConfig()

	logger := newLogger()

	tracer, closeFn, err := initTracer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &AlloyClient{
		Tracer:        tracer,
		Logger:        logger,
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

func (ac *AlloyClient) AddLog(ctx context.Context, level slog.Level, msg string, attrs ...any) error {
	_, span, err := ac.StartTrace(ctx, "log")
	if err != nil {
		return fmt.Errorf("failed to start tracing: %v", err)
	}

	span.SetAttributes(attribute.String(slogLevelToString(level), msg))
	span.End()

	spanCtx := span.SpanContext()
	if spanCtx.HasSpanID() && spanCtx.HasTraceID() {
		attrs = append(attrs, "trace_id", spanCtx.TraceID().String(), "span_id", spanCtx.SpanID().String())
	}

	if level == slog.LevelDebug {
		ac.Logger.Debug(msg, attrs...)
	} else if level == slog.LevelInfo {
		ac.Logger.Info(msg, attrs...)
	} else if level == slog.LevelWarn {
		ac.Logger.Warn(msg, attrs...)
	} else if level == slog.LevelError {
		ac.Logger.Error(msg, attrs...)
	}

	return nil
}

func (ac *AlloyClient) Shutdown(ctx context.Context) error {
	if ac.traceShutdown != nil {
		return ac.traceShutdown(ctx)
	}
	return nil
}

func initTracer(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	httpOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.TraceEndpoint),
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

func newLogger() *slog.Logger {
	today := time.Now().Format("2006-01-02")
	logDir := fmt.Sprintf("/var/log/alloy-interface/%s", today)
	file, err := os.OpenFile(logDir, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return nil
	}
	defer file.Close()

	handler := slog.NewJSONHandler(file, nil)
	return slog.New(handler)
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
