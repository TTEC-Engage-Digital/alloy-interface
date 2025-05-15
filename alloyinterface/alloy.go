package alloyinterface

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

func (ac *AlloyClient) AddSpanWithAttr(ctx context.Context, name string, attrs ...attribute.KeyValue) error {
	if ac.Tracer == nil {
		return errors.New("tracer not initialized")
	}
	_, span := ac.Tracer.Start(ctx, name)
	span.SetAttributes(attrs...)
	span.End()
	return nil
}

func (ac *AlloyClient) AddSpan(ctx context.Context, tracerName string, title string, msgBody string) error {
	_, span, err := ac.startTrace(ctx, tracerName)
	if err != nil {
		return fmt.Errorf("failed to start tracing: %v", err)
	}

	span.SetAttributes(attribute.String(title, msgBody))
	span.End()
	return nil
}

func (ac *AlloyClient) AddLog(ctx context.Context, level string, msg string) (*http.Response, error) {
	logRecord := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"log": map[string]string{
			"level":        level,
			"message":      msg,
			"is_secret":    "false",
			"service_name": ac.cfg.ServiceName,
		},
	}

	jsonBytes, err := json.Marshal(logRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log record: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ac.cfg.LogEndpoint+"/loki/api/v1/raw", bytes.NewBuffer(jsonBytes))
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

func (ac *AlloyClient) startTrace(ctx context.Context, name string) (context.Context, trace.Span, error) {
	if ac.Tracer == nil {
		return nil, nil, errors.New("tracer not initialized")
	}
	ctx, span := ac.Tracer.Start(ctx, name)
	return ctx, span, nil
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
