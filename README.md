# 📘 `alloyinterface` Package Documentation

`alloyinterface` is a Go package that simplifies setting up and using OpenTelemetry with [Grafana Alloy](https://grafana.com/docs/alloy/latest/) as a trace backend, using OTLP over HTTP.

---

## 📦 Overview

This package provides:

- Auto-configured OpenTelemetry `TracerProvider` with OTLP HTTP exporter.
- Environment-based configuration for endpoint and service metadata.
- Simple API to start and manage trace spans.
- Graceful shutdown of tracing system to flush remaining spans.
- Logging support with structured logs sent to a configurable endpoint.

---

## 🔧 Configuration

### 1. This package uses the following environment variables

| Variable Name           | Default Value          | Description                                 |
|-------------------------|------------------------|---------------------------------------------|
| `ALLOY_ENDPOINT`        | `localhost:4318`       | OTLP endpoint for Grafana Alloy             |
| `ALLOY_LOG_ENDPOINT`    | `http://localhost:9999`| Endpoint for sending logs to Grafana Alloy  |
| `ALLOY_SERVICE_NAME`    | `addi`                 | Service name for traces and logs            |
| `ALLOY_TRACER_NAME`     | `addi-tracer`          | Logical name of the tracer                  |

Example setup:

```bash
export ALLOY_ENDPOINT=localhost:4318
export ALLOY_LOG_ENDPOINT=http://localhost:9999
export ALLOY_SERVICE_NAME=my-service
export ALLOY_TRACER_NAME=my-service-tracer
```

---

### 2. Setup local Grafana Alloy

You can set up the local Grafana Alloy by following the instructions on the [Grafana Alloy documentation page](https://grafana.com/docs/alloy/latest/).

Or you can install alloy following the [integration](https://ttecdev.grafana.net/connections/add-new-connection/golang?page=alloy).

The sample command to install alloy is: (remember to update the id and api key)

```bash
GCLOUD_HOSTED_METRICS_ID="..." GCLOUD_HOSTED_METRICS_URL="https://prometheus-prod-10-prod-us-central-0.grafana.net/api/prom/push" GCLOUD_HOSTED_LOGS_ID="..." GCLOUD_HOSTED_LOGS_URL="https://logs-prod3.grafana.net/loki/api/v1/push" GCLOUD_RW_API_KEY="..." /bin/sh -c "$(curl -fsSL https://storage.googleapis.com/cloud-onboarding/alloy/scripts/install-linux.sh)"
```

Set up Grafana Alloy to use the Go integration
You can find your configuration file for your Alloy instance at /etc/alloy/config.alloy.

First, manually copy and replace the following snippets into your alloy configuration file.

```bash
livedebugging {
        enabled = true
}

loki.source.api "listener" {
    http {
        listen_address = "127.0.0.1"
        listen_port    = 9999
    }

    labels = { "source" = "api" }

    forward_to = [loki.process.process_logs.receiver]
}

loki.process "process_logs" {

    // Stage 1
    stage.json {
        expressions = {
            log = "",
            ts  = "timestamp",
        }
    }

    // Stage 2
    stage.timestamp {
        source = "ts"
        format = "RFC3339"
    }

    // Stage 3
    stage.json {
        source = "log"

        expressions = {
            is_secret    = "",
            level        = "",
            log_line     = "message",
            service_name = "",
        }
    }

    // Stage 4
    stage.drop {
        source = "is_secret"
        value  = "true"
    }

    // Stage 5
    stage.labels {
        values = {
            level        = "",
            service_name = "",
        }
    }

    // Stage 6
    stage.output {
        source = "log_line"
    }

    // This stage adds static values to the labels on the log line
    stage.static_labels {
        values = {
            source = "demo-api",  // replace this with the real name of source
        }
    }

    forward_to = [loki.write.grafana_cloud_loki.receiver]
}

loki.write "grafana_cloud_loki" {
        endpoint {
                url = "https://logs-prod3.grafana.net/loki/api/v1/push"

                basic_auth {
                        // replace this with your auth info. you can get it from [here - passowrd](https://grafana.com/orgs/ttec/hosted-logs/273608)
                        username = "..."
                        password = "glc_..."
                }
        }
}

otelcol.receiver.otlp "default" {
        grpc {}

        http {}

        output {
                traces  = [otelcol.processor.batch.default.input]
        }
}

otelcol.processor.batch "default" {
        output {
                metrics = [otelcol.exporter.otlphttp.grafana_cloud.input]
                traces  = [otelcol.exporter.otlphttp.grafana_cloud.input]
                logs    = [otelcol.exporter.otlphttp.grafana_cloud.input]
        }
}

otelcol.exporter.otlphttp "grafana_cloud" {
        client {
                endpoint = "https://otlp-gateway-prod-us-central-0.grafana.net/otlp"
                auth     = otelcol.auth.basic.grafana_cloud.handler
        }
}

otelcol.auth.basic "grafana_cloud" {
        // replace this with your auth info. you can get it from [here - instance id and password](https://grafana.com/orgs/ttec/stacks/424255/otlp-info)
        username = ... 
        password = "glc_..."
}
```

Restart Grafana Alloy and test configurations
Restart Grafana Alloy
Once you’ve changed your configuration file, run the following command to restart Grafana Alloy.

After installation, the config is stored in /etc/alloy/config.alloy. Restart Alloy for any changes to take effect:

```bash
sudo systemctl restart alloy.service
```

Now you can check if alloy is running by visiting the [alloy local website](http://localhost:12345)

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

### 🔹 `func (ac *AlloyClient) AddLog(ctx context.Context, level string, msg string) (*http.Response, error)`

Sends a structured log to the configured log endpoint.

**Parameters:**

- `ctx`: Context for the log request.
- `level`: Log level (e.g., `info`, `error`).
- `msg`: Log message.

**Example:**

```go
resp, err := client.AddLog(ctx, "info", "This is a test log message")
if err != nil {
    log.Printf("Failed to send log: %v", err)
}
```

---

### 🔹 `func (ac *AlloyClient) AddSpan(ctx context.Context, tracerName string, title string, msgBody string) error`

Creates and ends a span immediately under the given context, often used for sub-operations.

**Parameters:**

- `ctx`: Context with parent span.
- `tracerName`: Name of the tracer.
- `title`: Attribute key for the span.
- `msgBody`: Attribute value for the span.

**Example:**

```go
err := client.AddSpan(ctx, "db-query", "query", "SELECT * FROM users")
if err != nil {
    log.Printf("Failed to add span: %v", err)
}
```

---

### 🔹 `func (ac *AlloyClient) Shutdown(ctx context.Context) error`

Shuts down the tracer provider and flushes all spans.

**Should always be called before the program exits.**

**Example:**

```go
defer func() {
    if err := client.Shutdown(ctx); err != nil {
        log.Fatalf("Failed to shutdown Alloy client: %v", err)
    }
}()
```

---

## 📦 Dependencies

Install the required OpenTelemetry packages:

```bash
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
```

---

## 🛠️ Example Usage

```go
package main

import (
    "context"
    "log"

    "github.com/TTEC-Engage-Digital/alloy-interface/alloyinterface"
)

func main() {
    ctx := context.Background()

    client, err := alloyinterface.NewAlloyClient(ctx)
    if err != nil {
        log.Fatalf("Failed to create Alloy client: %v", err)
    }
    defer client.Shutdown(ctx)

    // Add a log
    err = client.AddLog(ctx, "info", "Application started")
    if err != nil {
        log.Printf("Failed to send log: %v", err)
    }

    // Add a span
    err = client.AddSpan(ctx, "main-operation", "operation", "processing data")
    if err != nil {
        log.Printf("Failed to add span: %v", err)
    }
}
```
