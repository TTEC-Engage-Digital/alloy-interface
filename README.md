# 📘 `alloyinterface` Package Documentation

`alloyinterface` is a Go package that simplifies setting up and using OpenTelemetry with [Grafana Alloy](https://grafana.com/docs/alloy/latest/) as a trace backend, using OTLP over HTTP.

---

## 📦 Overview

This package provides:

- Auto-configured OpenTelemetry `TracerProvider` with OTLP HTTP exporter.
- Environment-based configuration for endpoint and service metadata.
- Simple API to start and manage trace spans.
- Graceful shutdown of tracing system to flush remaining spans.

---

## 🔧 Configuration

### 1. This package uses the following environment variables:

| Variable Name           | Default Value      | Description                            |
|------------------------|--------------------|----------------------------------------|
| `ALLOY_ENDPOINT`       | `localhost:4318`   | OTLP endpoint for Grafana Alloy        |
| `ALLOY_SERVICE_NAME`   | `addi`             | Service name for traces                |
| `ALLOY_TRACER_NAME`    | `addi-tracer`      | Logical name of the tracer             |

Example setup:

```bash
export ALLOY_ENDPOINT=localhost:4318
export ALLOY_SERVICE_NAME=my-service
export ALLOY_TRACER_NAME=my-service-tracer
```

---

### 2. Setup local Grafana alloy (optional)

You can setup the local grafana alloy by following the below page
https://ttecdev.grafana.net/connections/infrastructure/golang?page=alloy

After setting up Grafana Alloy to use the Go integration, you need to add some more configuration in config.alloy file.
```bash
otelcol.receiver.otlp "default" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.receiver.otlp/

        // configures the default grpc endpoint "0.0.0.0:4317"
        grpc { }
        // configures the default http/protobuf endpoint "0.0.0.0:4318"
        http { }

        output {
                metrics = [otelcol.processor.resourcedetection.default.input]
                logs    = [otelcol.processor.resourcedetection.default.input]
                traces  = [otelcol.processor.resourcedetection.default.input]
        }
}

otelcol.processor.resourcedetection "default" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.processor.resourcedetection/
        detectors = ["env", "system"] // add "gcp", "ec2", "ecs", "elastic_beanstalk", "eks", "lambda", "azure", "aks",>
        system {
                hostname_sources = ["os"]
        }

		 output {
                metrics = [otelcol.processor.transform.drop_unneeded_resource_attributes.input]
                logs    = [otelcol.processor.transform.drop_unneeded_resource_attributes.input]
                traces  = [otelcol.processor.transform.drop_unneeded_resource_attributes.input]
        }
}

otelcol.processor.transform "drop_unneeded_resource_attributes" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.processor.transform/
        error_mode = "ignore"

        trace_statements {
                context    = "resource"
                statements = [
                        "delete_key(attributes, \"k8s.pod.start_time\")",
                        "delete_key(attributes, \"os.description\")",
                        "delete_key(attributes, \"os.type\")",
                        "delete_key(attributes, \"process.command_args\")",
                        "delete_key(attributes, \"process.executable.path\")",
                        "delete_key(attributes, \"process.pid\")",
                        "delete_key(attributes, \"process.runtime.description\")",
                        "delete_key(attributes, \"process.runtime.name\")",
                        "delete_key(attributes, \"process.runtime.version\")",
                ]
		}

        metric_statements {
                context    = "resource"
                statements = [
                        "delete_key(attributes, \"k8s.pod.start_time\")",
                        "delete_key(attributes, \"os.description\")",
                        "delete_key(attributes, \"os.type\")",
                        "delete_key(attributes, \"process.command_args\")",
                        "delete_key(attributes, \"process.executable.path\")",
                        "delete_key(attributes, \"process.pid\")",
                        "delete_key(attributes, \"process.runtime.description\")",
                        "delete_key(attributes, \"process.runtime.name\")",
                        "delete_key(attributes, \"process.runtime.version\")",
                ]
        }

		log_statements {
                context    = "resource"
                statements = [
                        "delete_key(attributes, \"k8s.pod.start_time\")",
                        "delete_key(attributes, \"os.description\")",
                        "delete_key(attributes, \"os.type\")",
                        "delete_key(attributes, \"process.command_args\")",
                        "delete_key(attributes, \"process.executable.path\")",
                        "delete_key(attributes, \"process.pid\")",
                        "delete_key(attributes, \"process.runtime.description\")",
                        "delete_key(attributes, \"process.runtime.name\")",
                        "delete_key(attributes, \"process.runtime.version\")",
                ]
        }

        output {
                metrics = [otelcol.processor.transform.add_resource_attributes_as_metric_attributes.input]
                logs    = [otelcol.processor.batch.default.input]
                traces  = [
                        otelcol.processor.batch.default.input,
                        otelcol.connector.host_info.default.input,
                ]
        }
}

otelcol.connector.host_info "default" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.connector.host_info/
        host_identifiers = ["host.name"]

        output {
                metrics = [otelcol.processor.batch.default.input]
        }
}

otelcol.processor.transform "add_resource_attributes_as_metric_attributes" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.processor.transform/
        error_mode = "ignore"

        metric_statements {
                context    = "datapoint"
                statements = [
                        "set(attributes[\"deployment.environment\"], resource.attributes[\"deployment.environment\"])",
                        "set(attributes[\"service.version\"], resource.attributes[\"service.version\"])",
                ]
        }

		output {
                metrics = [otelcol.processor.batch.default.input]
        }
}

otelcol.processor.batch "default" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.processor.batch/
        output {
                metrics = [otelcol.exporter.otlphttp.grafana_cloud.input]
                logs    = [otelcol.exporter.otlphttp.grafana_cloud.input]
                traces  = [otelcol.exporter.otlphttp.grafana_cloud.input]
        }
}

otelcol.exporter.otlphttp "grafana_cloud" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.otlphttp/
        client {
                endpoint = "https://otlp-gateway-prod-us-central-0.grafana.net/otlp"
                auth     = otelcol.auth.basic.grafana_cloud.handler
        }
}

otelcol.auth.basic "grafana_cloud" {
        // https://grafana.com/docs/alloy/latest/reference/components/otelcol.auth.basic/
        username = 0000000 // replace real username with this
        password = "replace real password"
}
```

---

## 🧩 Functions

### 🔹 `func NewAlloyClient(ctx context.Context) (*AlloyClient, error)`

Creates a new instance of `AlloyClient` with OpenTelemetry tracer initialized.

**Example:**

```go
ctx := context.Background()
client, err := alloyinterface.NewAlloyClient(ctx)
if err != nil {
 log.Fatalf("Failed to create AlloyClient: %v", err)
}
defer client.Shutdown(ctx)
```

---

### 🔹 `func (ac *AlloyClient) StartTrace(ctx context.Context, name string) (context.Context, trace.Span, error)`

Starts a new root span for a given operation name.

**Parameters:**
- `ctx`: the context to start from.
- `name`: name of the operation/span.

**Returns:**
- New context with the span.
- The span object.
- Error if the tracer isn't initialized.

**Example:**

```go
ctx, span, err := client.StartTrace(ctx, "main-operation")
if err != nil {
 log.Fatalf("Failed to start trace: %v", err)
}
defer span.End()
```

---

### 🔹 `func (ac *AlloyClient) AddSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) error`

Creates and ends a span immediately under the given context, often used for sub-operations.

**Parameters:**
- `ctx`: context with parent span.
- `name`: sub-span name.
- `attrs`: optional attributes to tag the span with.

**Example:**

```go
err := client.AddSpan(ctx, "db-query",
 attribute.String("db.system", "postgresql"),
 attribute.Int("rows_returned", 10),
)
```

---

### 🔹 `func (ac *AlloyClient) Shutdown(ctx context.Context) error`

Shuts down the tracer provider and flushes all spans.

**Should always be called before the program exits.**

**Example:**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/TTEC-Engage-Digital/alloy-interface/alloyinterface"
	"github.com/shirou/gopsutil/cpu"
	"go.opentelemetry.io/otel/attribute"
)

func monitorCPU(ctx context.Context, client *alloyinterface.AlloyClient) {
	for {
		percentages, err := cpu.Percent(time.Second, false)
		if err != nil {
			log.Printf("failed to get CPU usage: %v", err)
			continue
		}

		if len(percentages) > 0 {
			_, span, err := client.StartTrace(ctx, "cpu_usage")
			if err != nil {
				log.Printf("failed to start tracing: %v", err)
				return
			}

			span.SetAttributes(
				attribute.Float64("cpu.usage", percentages[0]),
			)
			span.End()
			log.Printf("CPU usage: %.2f%%", percentages[0])
		}
		time.Sleep(5 * time.Second)
	}
}

func main() {
	ctx := context.Background()

	client, err := alloyinterface.NewAlloyClient(ctx)
	if err != nil {
		log.Fatalf("failed to create Alloy client: %v", err)
	}

	defer func() {
		if err := client.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown Alloy client: %v", err)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		_, span, err := client.StartTrace(ctx, "http_request")
		if err != nil {
			http.Error(w, "failed to start trace", http.StatusInternalServerError)
			return
		}

		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // Simulate some work

		duration := time.Since(start)
		span.SetAttributes(
			attribute.Int64("response.time.ms", duration.Milliseconds()),
		)
		span.End()

		fmt.Fprintf(w, "pong (%dms)", duration.Milliseconds())
	})

	go monitorCPU(ctx, client)

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## 📦 Dependencies

Install the required OpenTelemetry packages:

```bash
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get github.com/TTEC-Engage-Digital/alloy-interface@latest
...
```
