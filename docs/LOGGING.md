# Logging and Collection

Sphere provides a flexible and powerful logging system built on top of `go.uber.org/zap`, a high-performance structured
logger. It supports multiple outputs, including console and file, and is designed for both local development and
production environments.

The core logging functionality can be found in the [`log`](../log) directory.

## Configuration

The logger is configured through the main `config.json` file in your project root (`layout/config.json`). The
configuration allows you to define log levels and outputs.

```json
{
  "log": {
    "level": "info",
    "console": {
      "disable": false
    },
    "file": {
      "file_name": "app.log",
      "max_size": 10,
      "max_backups": 5,
      "max_age": 7
    }
  }
}
```

### Configuration Fields

* `level`: The minimum log level to record (e.g., `debug`, `info`, `warn`, `error`).
* `console`: Console logging options.
    * `disable`: Set to `true` to turn off console output (default: `false`)
* `file`: File logging options. If this section is omitted, file logging is disabled.
    * `file_name`: The path to the log file.
    * `max_size`: The maximum size in megabytes of the log file before it gets rotated.
    * `max_backups`: The maximum number of old log files to retain.
    * `max_age`: The maximum number of days to retain old log files.

## Basic Usage

Sphere provides a global logger that can be used anywhere in your application.

### Standard Logging

You can use the global functions for standard logging:

```go
package main

import "github.com/go-sphere/sphere/log"

func main() {
	log.Debug("This is a debug message")
	log.Info("This is an info message", log.String("user", "test"))
	log.Warn("This is a warning")
	log.Error("This is an error", log.Err(fmt.Errorf("an error occurred")))
}
```

### Structured Logging

To add structured context to your logs, you can use `log.With` to create a new logger instance with predefined fields.

```go
logger := log.With(log.String("service", "UserService"), log.String("traceId", "xyz-123"))

logger.Info("User lookup successful")
// Output will include {"service": "UserService", "traceId": "xyz-123", "message": "User lookup successful"}
```

## Log Collection

While file and console outputs are useful for development, you'll need a more robust solution for production
environments. Sphere is compatible with various log collection tools.

### Simple Collection with Logdy

For simple, real-time log viewing without a complex setup, you can use [Logdy](https://github.com/logdyhq/logdy-core).
It's a lightweight tool that can stream logs directly to your browser.

If your application is writing logs to `app.log`, you can start Logdy with the following command:

```bash
tail -f app.log | logdy
#or
logdy follow app.log
```

This is ideal for debugging during development or monitoring a single instance.

### Advanced Collection with Grafana Loki

For a production-grade, scalable log aggregation system, Sphere recommends
using [Grafana Loki](https://grafana.com/oss/loki/). Loki is a horizontally scalable, multi-tenant log aggregation
system inspired by Prometheus.

The project template includes a pre-configured [`docker-compose.yaml`](../layout/devops/loki/docker-compose.yaml) to
quickly set up a local Loki and Grafana stack.

To start the stack, navigate to `layout/devops/loki/` and run:

```bash
docker-compose up -d
```

This will start three services:

* **Loki**: The log storage and query engine, available at `http://localhost:3100`.
* **Promtail**: The agent that collects logs and sends them to Loki.
* **Grafana**: The visualization dashboard, available at `http://localhost:3000`.

The included Grafana instance is pre-configured with Loki as a data source, so you can immediately start exploring your
logs.
