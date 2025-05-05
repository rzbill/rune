package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/rzbill/rune/pkg/log"
)

func main() {
	// Create a logger with the improved text formatter
	logger := log.NewLogger(
		log.WithLevel(log.DebugLevel),
		log.WithFormatter(&log.TextFormatter{
			ShortTimestamp: false,
			DisableColors:  false,
			EnableCaller:   false,
		}),
	)

	// Set as the default logger
	log.SetDefaultLogger(logger)

	// Basic examples with the new Field-based API
	log.Info("Application starting", log.Component("app"))

	log.Debug("Debug message with fields",
		log.Str("user", "admin"),
		log.Int("attempt", 1),
		log.Bool("success", true))

	// Error handling with the Err helper
	if err := simulateError(); err != nil {
		log.Error("Failed to process request",
			log.Err(err),
			log.Component("processor"),
			log.Int("status", 500))
	}

	// Context with trace information
	ctx := context.Background()
	ctx = context.WithValue(ctx, log.RequestIDKey, "req-12345")
	ctx = context.WithValue(ctx, log.TraceIDKey, "trace-abcdef")
	ctx = context.WithValue(ctx, log.SpanIDKey, "span-123456")

	// Operation with context and timing
	performOperation(ctx)

	// Using With to create a component logger
	apiLogger := log.With(log.Component("api-server"))

	// Log various message types
	apiLogger.Info("API server initialized")
	apiLogger.Info("Server listening", log.Int("port", 8080))

	// Process a request
	processRequest(apiLogger)

	// Simulate different log levels
	logAllLevels()
}

func simulateError() error {
	return errors.New("connection timeout")
}

func performOperation(ctx context.Context) {
	logger := log.FromContext(ctx).With(
		log.Component("worker"),
		log.Str("operation", "data-processing"))

	logger.Info("Starting operation")

	start := time.Now()
	// Simulate work
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))

	logger.Info("Operation completed",
		log.Duration("duration", time.Since(start)),
		log.Int("records_processed", 42))
}

func processRequest(logger log.Logger) {
	requestID := fmt.Sprintf("%d", rand.Int63())

	// Add request context to the logger
	reqLogger := logger.With(
		log.RequestID(requestID),
		log.Str("method", "GET"),
		log.Str("path", "/api/users"))

	reqLogger.Info("Request started")

	// Simulate processing
	start := time.Now()
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)))

	// Log completion with duration and status
	reqLogger.Info("Request completed",
		log.Int("status", 200),
		log.Duration("duration_ms", time.Since(start)))
}

func logAllLevels() {
	component := log.Component("demo")

	log.Debug("This is a debug message", component)
	log.Info("This is an info message", component)
	log.Warn("This is a warning message", component)
	log.Error("This is an error message", component)

	// Uncomment to see fatal (will exit the program)
	// log.Fatal("This is a fatal message", component)
}
