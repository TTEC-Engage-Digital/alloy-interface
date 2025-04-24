package alloyinterface

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type AlloyClient struct {
	Tracer trace.Tracer
	cfg    Config
	close  func(context.Context) error
}

func NewAlloyClient(ctx context.Context) (*AlloyClient, error) {
	cfg := LoadConfig()
	tracer, closeFn, err := initTracer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &AlloyClient{
		Tracer: tracer,
		cfg:    cfg,
		close:  closeFn,
	}, nil
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

func (ac *AlloyClient) Shutdown(ctx context.Context) error {
	if ac.close != nil {
		return ac.close(ctx)
	}
	return nil
}
