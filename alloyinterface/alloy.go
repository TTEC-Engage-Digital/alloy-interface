package alloyinterface

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
	"gopkg.in/natefinch/lumberjack.v2"
)

type AlloyClient struct {
	Tracer        trace.Tracer
	Logger        zerolog.Logger
	cfg           Config
	traceShutdown func(context.Context) error
	rateLimiter   *rate.Limiter
}

// Allow injection for testing
var (
	initTracerFn = initTracer
	initLogFn    = initLog
)

func NewAlloyClient(ctx context.Context) (*AlloyClient, error) {
	cfg := LoadConfig()

	tracer, closeFn, err := initTracerFn(ctx, cfg)
	if err != nil {
		return nil, err
	}

	logger, err := initLogFn()
	if err != nil {
		return nil, err
	}

	// Initialize the rate limiter with a limit of 10 requests per second and a burst size of 20
	rateLimiter := rate.NewLimiter(10, 20)

	return &AlloyClient{
		Tracer:        tracer,
		Logger:        logger,
		cfg:           cfg,
		traceShutdown: closeFn,
		rateLimiter:   rateLimiter,
	}, nil
}

func (ac *AlloyClient) AddSpanWithAttr(ctx context.Context, tracerName string, attrs ...attribute.KeyValue) error {
	if ac.Tracer == nil {
		ac.Logger.Error().Msg("AddSpanWithAttr: Tracer not initialized")
		return errors.New("tracer not initialized")
	}

	ac.Logger.Info().
		Str("tracer_name", tracerName).
		Int("attributes_count", len(attrs)).
		Msg("AddSpanWithAttr: Starting span with attributes")

	_, span, err := ac.startTrace(ctx, tracerName)
	if err != nil {
		ac.Logger.Error().
			Err(err).
			Str("tracer_name", tracerName).
			Msg("AddSpanWithAttr: Failed to start tracing")
		return fmt.Errorf("failed to start tracing: %v", err)
	}
	span.SetAttributes(attrs...)
	span.End()

	ac.Logger.Info().
		Str("tracer_name", tracerName).
		Msg("AddSpanWithAttr: Span with attributes ended successfully")

	return nil
}

func (ac *AlloyClient) AddSpan(ctx context.Context, tracerName string, title string, msgBody string) error {
	if ac.Tracer == nil {
		ac.Logger.Error().Msg("AddSpan: Tracer not initialized")
		return errors.New("tracer not initialized")
	}

	if tracerName == "" {
		ac.Logger.Error().Msg("AddSpan: Tracer name cannot be empty")
		return errors.New("tracer name cannot be empty")
	}

	ac.Logger.Info().
		Str("tracer_name", tracerName).
		Str("title", title).
		Msg("AddSpan: Starting span")

	_, span, err := ac.startTrace(ctx, tracerName)
	if err != nil {
		ac.Logger.Error().
			Err(err).
			Str("tracer_name", tracerName).
			Msg("AddSpan: Failed to start tracing")
		return fmt.Errorf("failed to start tracing: %v", err)
	}

	span.SetAttributes(attribute.String(title, msgBody))
	span.End()

	ac.Logger.Info().
		Str("tracer_name", tracerName).
		Str("title", title).
		Msg("AddSpan: Span ended successfully")

	return nil
}

func (ac *AlloyClient) AddLog(ctx context.Context, level zerolog.Level, msg string) (*http.Response, error) {
	if err := ac.rateLimiter.Wait(ctx); err != nil {
		ac.Logger.Error().Err(err).Msg("AddLog: Rate limit exceeded")
		return nil, fmt.Errorf("rate limit exceeded: %v", err)
	}

	if level < zerolog.DebugLevel || level > zerolog.PanicLevel {
		ac.Logger.Error().Msg("AddLog: invalid log level")
		return nil, errors.New("invalid log level")
	}
	if len(msg) == 0 {
		ac.Logger.Error().Msg("AddLog: Log message cannot be empty")
		return nil, errors.New("log message cannot be empty")
	}

	requestID := ctx.Value("request_id")
	if requestID == nil {
		requestID = "unknown"
	}

	logRecord := map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"level":        level,
		"message":      msg,
		"is_secret":    "false",
		"service_name": ac.cfg.ServiceName,
		"request_id":   requestID,
	}

	jsonBytes, err := json.Marshal(logRecord)
	if err != nil {
		ac.Logger.Error().Err(err).Msg("AddLog: failed to marshal log record")
		return nil, fmt.Errorf("failed to marshal log record: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ac.cfg.LogEndpoint+"/loki/api/v1/raw", bytes.NewBuffer(jsonBytes))
	if err != nil {
		ac.Logger.Error().Err(err).Msg("AddLog: failed to create srequest")
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ac.Logger.Info().
		Str("Status", "Preparing to send logs").
		Str("level", level.String()).
		Str("service_name", ac.cfg.ServiceName).
		Str("request_id", fmt.Sprintf("%v", requestID)).
		Msg("AddLog: " + msg)

	resp, err := client.Do(req)
	if err != nil {
		ac.Logger.Error().
			Err(err).
			Str("service_name", ac.cfg.ServiceName).
			Str("request_id", fmt.Sprintf("%v", requestID)).
			Msg("AddLog: Failed to send HTTP request")
		return resp, fmt.Errorf("failed to send request: %v", err)
	}

	if resp != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode >= 300 {
		ac.Logger.Error().
			Int("status_code", resp.StatusCode).
			Str("service_name", ac.cfg.ServiceName).
			Str("request_id", fmt.Sprintf("%v", requestID)).
			Msg("AddLog: Received non-success status code from log endpoint")
		return resp, fmt.Errorf("failed to send log record, status code: %d", resp.StatusCode)
	}

	ac.Logger.Info().
		Int("status_code", resp.StatusCode).
		Str("service_name", ac.cfg.ServiceName).
		Str("request_id", fmt.Sprintf("%v", requestID)).
		Msg("AddLog: Log record sent successfully")

	return resp, nil
}

func (ac *AlloyClient) SetRateLimit(limit rate.Limit, burst int) {
	if ac.rateLimiter == nil {
		ac.rateLimiter = rate.NewLimiter(limit, burst)
	}

	if limit > 0 {
		ac.rateLimiter.SetLimit(limit)
	}
	if burst > 0 {
		ac.rateLimiter.SetBurst(burst)
	}
}

func (ac *AlloyClient) Shutdown(ctx context.Context) error {
	var shutdownErrs []error
	if ac.traceShutdown != nil {
		if err := ac.traceShutdown(ctx); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("failed to shutdown tracer: %v", err))
		}
	}

	if len(shutdownErrs) > 0 {
		ac.Logger.Error().Err(fmt.Errorf("%v", shutdownErrs)).Msg("Shutdown: Errors occurred during shutdown")
		return fmt.Errorf("shutdown errors: %v", shutdownErrs)
	}
	return nil
}

func initTracer(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error

	httpOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.TraceEndpoint),
		otlptracehttp.WithTLSClientConfig(&tls.Config{InsecureSkipVerify: false}),
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

func initLog() (zerolog.Logger, error) {
	// Ensure the logs directory exists
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return zerolog.Logger{}, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Initialize the logger
	return zerolog.New(zerolog.ConsoleWriter{
		Out: &lumberjack.Logger{
			Filename:   logDir + "/alloy.log",
			MaxSize:    10, // Max megabytes before log is rotated
			MaxBackups: 3,  // Max number of old log files to keep
			MaxAge:     28, // Max number of days to retain old log files
			Compress:   true,
		},
	}).With().Timestamp().Logger(), nil
}

func (ac *AlloyClient) startTrace(ctx context.Context, name string) (context.Context, trace.Span, error) {
	if ac.Tracer == nil {
		return nil, nil, errors.New("tracer not initialized")
	}
	ctx, span := ac.Tracer.Start(ctx, name)
	return ctx, span, nil
}
