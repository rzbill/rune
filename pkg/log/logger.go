// Package log provides a structured logging system for Rune services.
package log

import (
	"context"
	"time"
)

// Level represents the severity level of a log message.
type Level int

// Log levels
const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Fields is a map of field names to values.
type Fields map[string]interface{}

// Context keys for propagating logging context
const (
	RequestIDKey = "request_id"
	TraceIDKey   = "trace_id"
	SpanIDKey    = "span_id"
	ComponentKey = "component"
	OperationKey = "operation"
)

// Entry represents a single log entry.
type Entry struct {
	Level     Level
	Message   string
	Fields    Fields
	Timestamp time.Time
	Caller    string
	Error     error
}

// Logger defines the core logging interface for Rune components.
type Logger interface {
	// Standard logging methods with structured context (Field-based API)
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// Standard logging methods with key-value pairs (for backward compatibility)
	Debugf(msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Fatalf(msg string, args ...interface{})

	// Field creation methods (for backward compatibility)
	WithField(key string, value interface{}) Logger
	WithFields(fields Fields) Logger
	WithError(err error) Logger

	// With adds multiple fields to the logger (for new Field-based API)
	With(fields ...Field) Logger

	// WithContext adds request context to the Logger
	WithContext(ctx context.Context) Logger

	// WithComponent tags logs with a component name
	WithComponent(component string) Logger

	// SetLevel sets the minimum log level
	SetLevel(level Level)

	// GetLevel returns the current minimum log level
	GetLevel() Level
}

// Formatter defines the interface for formatting log entries.
type Formatter interface {
	Format(entry *Entry) ([]byte, error)
}

// Output defines the interface for log outputs.
type Output interface {
	Write(entry *Entry, formattedEntry []byte) error
	Close() error
}

// LoggerOption is a function that configures a logger.
type LoggerOption func(*BaseLogger)

// BaseLogger implements the Logger interface.
type BaseLogger struct {
	level     Level
	fields    Fields
	formatter Formatter
	outputs   []Output
	hooks     []Hook
}

// Hook is a function that is called during logging.
type Hook interface {
	Levels() []Level
	Fire(entry *Entry) error
}

// ContextExtractor extracts logging context from a context.Context.
func ContextExtractor(ctx context.Context) Fields {
	if ctx == nil {
		return Fields{}
	}

	fields := Fields{}

	// Extract standard context values
	if v := ctx.Value(RequestIDKey); v != nil {
		fields[RequestIDKey] = v
	}
	if v := ctx.Value(TraceIDKey); v != nil {
		fields[TraceIDKey] = v
	}
	if v := ctx.Value(SpanIDKey); v != nil {
		fields[SpanIDKey] = v
	}
	if v := ctx.Value(ComponentKey); v != nil {
		fields[ComponentKey] = v
	}
	if v := ctx.Value(OperationKey); v != nil {
		fields[OperationKey] = v
	}

	// Extract custom field keys (injected by ContextInjector)
	// We need to scan all context keys to find our custom fieldKeyType keys
	// This is a limitation of Go's context package - we can't enumerate all keys
	// For now, we'll rely on the standard keys above and any custom extraction logic

	return fields
}

// ContextInjector injects logging fields into a context.Context.
func ContextInjector(ctx context.Context, fields Fields) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	for k, v := range fields {
		ctx = context.WithValue(ctx, makeFieldKey(k), v)
	}

	return ctx
}

// FromContext extracts a logger from a context.Context.
// If no logger is found, it returns the default logger.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return defaultLogger
	}

	if logger, ok := ctx.Value(loggerKey).(Logger); ok {
		return logger
	}

	// Extract context fields and add them to the default logger
	fields := ContextExtractor(ctx)
	if len(fields) > 0 {
		return defaultLogger.WithFields(fields)
	}

	return defaultLogger
}

// loggerKey is the context key for the logger.
type loggerKeyType struct{}

var loggerKey = loggerKeyType{}

// fieldKeyType is the context key type for logging fields to avoid collisions
type fieldKeyType struct {
	key string
}

// makeFieldKey creates a unique context key for a field
func makeFieldKey(key string) fieldKeyType {
	return fieldKeyType{key: key}
}

// WithLogger adds a logger to a context.Context.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Global default logger
var defaultLogger Logger

// Initialize the default logger
func init() {
	// Create a default logger that outputs to stderr with INFO level
	defaultLogger = NewLogger(WithLevel(InfoLevel))
}

// SetDefaultLogger sets the global default logger.
func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

// GetDefaultLogger returns the global default logger.
func GetDefaultLogger() Logger {
	return defaultLogger
}

// Standard global logging methods with fields
func Debug(msg string, fields ...Field) {
	defaultLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	defaultLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	defaultLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	defaultLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	defaultLogger.Fatal(msg, fields...)
}

// Standard global logging methods with key-value pairs (for backward compatibility)
func Debugf(msg string, args ...interface{}) {
	defaultLogger.Debugf(msg, args...)
}

func Infof(msg string, args ...interface{}) {
	defaultLogger.Infof(msg, args...)
}

func Warnf(msg string, args ...interface{}) {
	defaultLogger.Warnf(msg, args...)
}

func Errorf(msg string, args ...interface{}) {
	defaultLogger.Errorf(msg, args...)
}

func Fatalf(msg string, args ...interface{}) {
	defaultLogger.Fatalf(msg, args...)
}

// With adds fields to the logger (for new Field-based API)
func With(fields ...Field) Logger {
	return defaultLogger.With(fields...)
}

// For backward compatibility
func WithField(key string, value interface{}) Logger {
	return defaultLogger.WithField(key, value)
}

func WithFields(fields Fields) Logger {
	return defaultLogger.WithFields(fields)
}

func WithError(err error) Logger {
	return defaultLogger.WithError(err)
}

func WithContext(ctx context.Context) Logger {
	return defaultLogger.WithContext(ctx)
}

func WithComponent(component string) Logger {
	return defaultLogger.WithComponent(component)
}

// NewLogger creates a new logger with the given options.
func NewLogger(options ...LoggerOption) Logger {
	logger := &BaseLogger{
		level:     InfoLevel,
		fields:    Fields{},
		formatter: &JSONFormatter{},
		outputs:   []Output{},
		hooks:     []Hook{},
	}

	// Apply options
	for _, option := range options {
		option(logger)
	}

	// Add default output if none specified
	if len(logger.outputs) == 0 {
		logger.outputs = append(logger.outputs, &ConsoleOutput{})
	}

	return logger
}

// WithLevel sets the minimum log level.
func WithLevel(level Level) LoggerOption {
	return func(l *BaseLogger) {
		l.level = level
	}
}

// WithFormatter sets the log formatter.
func WithFormatter(formatter Formatter) LoggerOption {
	return func(l *BaseLogger) {
		l.formatter = formatter
	}
}

// WithOutput adds an output to the logger.
func WithOutput(output Output) LoggerOption {
	return func(l *BaseLogger) {
		l.outputs = append(l.outputs, output)
	}
}

// WithHook adds a hook to the logger.
func WithHook(hook Hook) LoggerOption {
	return func(l *BaseLogger) {
		l.hooks = append(l.hooks, hook)
	}
}
