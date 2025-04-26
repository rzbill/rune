# Rune Logging System

The Rune logging system provides a comprehensive, structured logging framework for all components of the Rune orchestration platform. It supports multiple log levels, structured data, context propagation, and various output formats and destinations.

## Key Features

- **Structured Logging**: Log entries include structured data as key-value pairs
- **Type-Safe Field API**: Strongly-typed field helpers for better developer experience
- **Multiple Log Levels**: DEBUG, INFO, WARN, ERROR, FATAL
- **Context Propagation**: Trace request flows across distributed components
- **Multiple Output Formats**: JSON, text with colored formatting
- **Multiple Output Destinations**: Console, files (with rotation), extensible for others
- **Performance Optimized**: Minimal overhead with support for sampling high-volume logs
- **Security Features**: Redaction of sensitive fields
- **Distributed Tracing**: OpenTelemetry compatibility (placeholder)

## Basic Usage with Field API (Recommended)

```go
import "github.com/razorbill/rune/pkg/log"

// Simple logging with fields
log.Info("Server starting", log.Int("port", 8080))

// Multiple fields with type safety
log.Info("Request completed",
    log.Component("http-server"),
    log.Int("status", 200),
    log.Str("method", "GET"),
    log.Str("path", "/users"),
    log.Duration("latency_ms", time.Since(start)))

// Error logging with contextual information
if err := someOperation(); err != nil {
    log.Error("Operation failed", 
        log.Err(err),
        log.Str("operation", "someOperation"),
        log.Component("processor"))
}
```

## Component-Specific Loggers

Create loggers specific to components for better filtering and organization:

```go
// Create a component-specific logger
apiLogger := log.With(log.Component("api-server"))

// Use the component logger
apiLogger.Info("API server starting")
apiLogger.Info("Route registered", log.Str("path", "/users"))
```

## Request Context Propagation

Trace requests across components with context propagation:

```go
// In HTTP handler middleware
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Generate or extract request ID
        reqID := r.Header.Get("X-Request-ID")
        if reqID == "" {
            reqID = uuid.New().String()
        }
        
        // Add request ID to context
        ctx := context.WithValue(r.Context(), log.RequestIDKey, reqID)
        
        // Add logger with request ID to context
        requestLogger := log.With(log.RequestID(reqID))
        ctx = log.WithLogger(ctx, requestLogger)
        
        // Log incoming request
        requestLogger.Info("Request started",
            log.Str("method", r.Method),
            log.Str("path", r.URL.Path),
            log.Str("remote_addr", r.RemoteAddr))
        
        start := time.Now()
        
        // Call next handler with updated context
        next.ServeHTTP(w, r.WithContext(ctx))
        
        // Log completion with duration
        requestLogger.Info("Request completed",
            log.Int("status", getStatusCode(w)),
            log.Duration("duration_ms", time.Since(start)))
    })
}

// In another component that receives the context
func ProcessRequest(ctx context.Context) {
    // Get logger from context with all the request tracking fields
    logger := log.FromContext(ctx)
    
    logger.Info("Processing request")
    // ... processing logic ...
    logger.Info("Request processed successfully", 
        log.Int("items_processed", 42))
}
```

## Available Field Types

The logging system provides type-safe field constructors:

```go
log.Str("key", "value")      // String field
log.Int("key", 123)          // Integer field
log.Int64("key", 123)        // Int64 field
log.Float64("key", 123.45)   // Float64 field
log.Bool("key", true)        // Boolean field
log.Err(err)                 // Error field
log.Time("key", time.Now())  // Time field
log.Duration("key", duration) // Duration field
log.Any("key", value)        // Any value type
log.F("key", value)          // Generic field (alias for Any)

// Context-specific field helpers
log.Component("component-name")  // Component identifier
log.RequestID("req-id")          // Request ID
log.TraceID("trace-id")          // Trace ID
log.SpanID("span-id")            // Span ID
```

## Configuration

Configure logging via code:

```go
logger := log.NewLogger(
    log.WithLevel(log.InfoLevel),
    log.WithFormatter(&log.TextFormatter{
        FullTimestamp: true,
        DisableColors: false,
    }),
    log.WithOutput(log.NewFileOutput("/var/log/rune/service.log", 
        log.WithMaxSize(100*1024*1024), // 100MB
        log.WithMaxBackups(5),
    )),
)

// Set as default logger
log.SetDefaultLogger(logger)
```

Or via configuration:

```go
config := &log.Config{
    Level:  "info",
    Format: "json",
    Outputs: []log.OutputConfig{
        {
            Type: "file",
            Options: map[string]interface{}{
                "filename":    "/var/log/rune/service.log",
                "max_size":    "100MB",
                "max_backups": 5,
            },
        },
        {
            Type: "console",
            Options: map[string]interface{}{
                "error_to_stderr": true,
            },
        },
    },
    RedactedFields: []string{"password", "api_key", "token"},
}

logger, err := log.ApplyConfig(config)
if err != nil {
    panic(err)
}

log.SetDefaultLogger(logger)
```

## Legacy API (Not Recommended for New Code)

The original key-value pair API is still supported for backward compatibility:

```go
// These methods use the "f" suffix to indicate they use the older format
log.Infof("Server starting", "port", 8080)
log.Errorf("Operation failed", "error", err.Error(), "operation", "someOperation")

// WithField/WithFields for the older API
logger := log.WithField("component", "api-server")
logger.Infof("Server starting")
```

## Performance Considerations

- Use appropriate log levels (DEBUG for development, INFO or higher for production)
- Consider sampling for high-volume logs
- Use structured fields consistently for easier parsing and querying
- Close file outputs properly when the application exits

## Best Practices

1. **Use the Field API**: The Field-based API provides better type safety and IDE support
2. **Tag with Component**: Always tag logs with a component name using `log.Component()`
3. **Propagate Context**: Always propagate request context with IDs
4. **Add Context to Errors**: Always add context to error logs with `log.Err(err)`
5. **Use Semantic Field Names**: Use consistent, semantic field names
6. **Security**: Use `RedactedFields` for sensitive data
7. **Resource Cleanup**: Close file outputs properly 