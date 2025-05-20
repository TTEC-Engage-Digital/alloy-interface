# 📘 `alloyinterface` Package Documentation

`alloyinterface` is a Go package that simplifies setting up and using OpenTelemetry with [Grafana Alloy](https://grafana.com/docs/alloy/latest/) as a trace backend, using OTLP over HTTP.

---

## 📦 Overview

This package provides:

- Pre-configured OpenTelemetry `TracerProvider` with an OTLP HTTP exporter.
- Environment-based configuration for endpoints and service metadata.
- A simple API to start and manage trace spans.
- Graceful shutdown of the tracing system to flush remaining spans.
- Structured logging support with logs sent to a configurable endpoint.

---

## 🔧 Configuration

### 1. Environment Variables Used by This Package

| Variable Name           | Default Value                         | Description                                 |
|-------------------------|---------------------------------------|---------------------------------------------|
| `ALLOY_ENDPOINT`        | `localhost:4318`                      | OTLP endpoint for Grafana Alloy             |
| `ALLOY_LOG_ENDPOINT`    | `http://localhost:9999`               | Endpoint for sending logs to Grafana Alloy  |
| `ALLOY_SERVICE_NAME`    | `addi`                                | Service name for traces and logs            |
| `ALLOY_TRACER_NAME`     | `addi-tracer`                         | Logical name of the tracer                  |
| `ALLOY_CERTFILE_PATH`   | `/etc/config/grafana-alloy.crt`       | Path to the certificate file for sending logs to Alloy |

Example setup:

```bash
export ALLOY_ENDPOINT=localhost:4318
export ALLOY_LOG_ENDPOINT=http://localhost:9999
export ALLOY_SERVICE_NAME=my-service
export ALLOY_TRACER_NAME=my-service-tracer
export ALLOY_CERTFILE_PATH="/etc/config/grafana-alloy.crt"
```

---

### 2. Setting Up Grafana Alloy and Configurations

#### Setting Up Grafana Alloy for Go Integration

You can set up Grafana Alloy locally by following the instructions on the [Grafana Alloy documentation page](https://grafana.com/docs/alloy/latest/).

Alternatively, you can install Alloy using the [integration guide](https://ttecdev.grafana.net/connections/add-new-connection/golang?page=alloy).

Here is a sample command to install Alloy (make sure to update the ID and API key):

```bash
GCLOUD_HOSTED_METRICS_ID="..." GCLOUD_HOSTED_METRICS_URL="https://prometheus-prod-10-prod-us-central-0.grafana.net/api/prom/push" GCLOUD_HOSTED_LOGS_ID="..." GCLOUD_HOSTED_LOGS_URL="https://logs-prod3.grafana.net/loki/api/v1/push" GCLOUD_RW_API_KEY="..." /bin/sh -c "$(curl -fsSL https://storage.googleapis.com/cloud-onboarding/alloy/scripts/install-linux.sh)"
```

#### Configuring Alloy

You can find the configuration file for your Alloy instance at `/etc/alloy/config.alloy`.

Manually copy and replace the following snippets into your Alloy configuration file:

```bash
loki.source.api "listener" {
    http {
        listen_address = "127.0.0.1"
        listen_port    = 9999
    }

    labels = { "source" = "api" }

    forward_to = [loki.process.process_logs.receiver]
}

loki.process "process_logs" {
    // Stage 1: Parse the entire log line as JSON
    stage.json {
        expressions = {
            ts           = "timestamp",
            level        = "",
            log_line     = "message",
            is_secret    = "",
            service_name = "",
            request_id   = "",
        }
    }

    // Stage 2: Parse timestamp from `ts`
    stage.timestamp {
        source = "ts"
        format = "RFC3339"
    }

    // Stage 3: Drop secret logs
    stage.drop {
        source = "is_secret"
        value  = "true"
    }

    // Stage 4: Set labels
    stage.labels {
        values = {
            level        = "",
            service_name = "",
            request_id   = "",
        }
    }

    // Stage 5: Set output to the message
    stage.output {
        source = "log_line"
    }

    // Stage 6: Add static labels
    stage.static_labels {
        values = {
            source = "demo-api",
        }
    }

    forward_to = [loki.write.grafana_cloud_loki.receiver]
}

loki.write "grafana_cloud_loki" {
        endpoint {
                url = "https://logs-prod3.grafana.net/loki/api/v1/push"

                basic_auth {
                        // Replace this with your authentication info. You can get it from [here - password](https://grafana.com/orgs/ttec/hosted-logs/273608)
                        username = "..."
                        password = "glc_..."
                }
        }
}

otelcol.receiver.otlp "default" {
        grpc {}

        http {
                tls {
                        cert_file = "/etc/config/grafana-alloy.crt"
                        key_file = "/etc/config/grafana-alloy.key"
                }
        }

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
        // Replace this with your authentication info. You can get it from [here - instance ID and password](https://grafana.com/orgs/ttec/stacks/424255/otlp-info)
        username = ... 
        password = "glc_..."
}
```

#### Generating Certificates

After updating your configuration file, run the following commands to generate the certificates:

```bash
touch cert.conf
nano cert.conf
```

Create a configuration file for generating certificates. Copy and paste the following content into the `cert.conf` file:

```bash
[req]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = dn
x509_extensions    = v3_req

[dn]
CN = localhost

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
```

Next, generate the certificates:

```bash
openssl req -x509 -newkey rsa:2048 -nodes   -keyout grafana-alloy.key   -out grafana-alloy.crt   -days 365   -config cert.conf
```

This will generate `grafana-alloy.key` and `grafana-alloy.crt`.

Move the certificates to the configuration folder:

```bash
sudo mkdir -p /etc/config
sudo cp grafana-alloy.crt /etc/config/
sudo cp grafana-alloy.key /etc/config/
sudo chown -R alloy:alloy /etc/config
```

#### Restarting Grafana Alloy and Testing Configurations

Restart Alloy to apply the changes:

```bash
sudo systemctl restart alloy.service
```

Verify that Alloy is running by visiting the [local Alloy website](http://localhost:12345).

---

## 🧩 Functions

### 🔹 `func NewAlloyClient(ctx context.Context) (*AlloyClient, error)`

Creates a new instance of `AlloyClient` with an OpenTelemetry tracer initialized.

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
- `level`: Log level (e.g., zerolog.InfoLevel, zerolog.DebugLevel).
- `msg`: Log message.

**Example:**

```go
requestID := uuid.New().String()
ctxWithRequestID := context.WithValue(r.Context(), "request_id", requestID)
resp, err := client.AddLog(ctxWithRequestID, zerolog.InfoLevel, "This is a test log message")
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
requestID := uuid.New().String()
ctxWithRequestID := context.WithValue(r.Context(), "request_id", requestID)
err := client.AddSpan(ctxWithRequestID, "db-query", "query", "SELECT * FROM users")
if err != nil {
    log.Printf("Failed to add span: %v", err)
}
```

---

### 🔹 `func (ac *AlloyClient) AddSpanWithAttr(ctx context.Context, tracerName string, attrs ...attribute.KeyValue) error`

Creates and ends a span immediately under the given context, often used for sub-operations.

**Parameters:**

- `ctx`: Context with parent span.
- `tracerName`: Name of the tracer.
- `attrs`: Attributes for the span.

**Example:**

```go
requestID := uuid.New().String()
ctxWithRequestID := context.WithValue(r.Context(), "request_id", requestID)
err := client.AddSpanWithAttr(ctxWithRequestID, "db-query", attribute.String("query", "SELECT * FROM users"))
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
    ...
)

func main() {
    ctx := context.Background()

    client, err := alloyinterface.NewAlloyClient(ctx)
    if err != nil {
        log.Fatalf("Failed to create Alloy client: %v", err)
    }
    defer func() {
        if err := client.Shutdown(ctx); err != nil {
            log.Fatalf("failed to shutdown Alloy client: %v", err)
        }
    }()

    // Add a span
    err = client.AddSpan(ctxWithRequestID, "main-operation", "operation", "processing data")
    if err != nil {
        log.Printf("Failed to add span: %v", err)
    }

    // Add a log
    requestID := uuid.New().String()
    ctxWithRequestID := context.WithValue(r.Context(), "request_id", requestID)
    resp, err := client.AddLog(ctxWithRequestID, zerolog.InfoLevel, "Application started")
    if err != nil {
        log.Printf("failed to send CPU usage log: %v", err)
    }
    log.Printf("Log sent: %v", resp)
}
```
