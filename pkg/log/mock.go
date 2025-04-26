package log

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// TestEntry represents a captured log entry for testing
type TestEntry struct {
	Level   Level
	Message string
	Fields  []Field
}

// TestLogger is a Logger implementation for testing that captures logs
// without producing output and provides methods to verify logging behavior.
type TestLogger struct {
	mu      sync.Mutex
	entries []TestEntry
	fields  []Field
	level   Level
}

// NewTestLogger creates a new TestLogger for use in unit tests
func NewTestLogger() *TestLogger {
	return &TestLogger{
		level: InfoLevel,
	}
}

// GetEntries returns all captured log entries
func (l *TestLogger) GetEntries() []TestEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Return a copy to prevent modification
	result := make([]TestEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// ClearEntries clears all captured log entries
func (l *TestLogger) ClearEntries() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// Debug logs a debug message
func (l *TestLogger) Debug(msg string, fields ...Field) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, fields)
	}
}

// Info logs an info message
func (l *TestLogger) Info(msg string, fields ...Field) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, fields)
	}
}

// Warn logs a warning message
func (l *TestLogger) Warn(msg string, fields ...Field) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, fields)
	}
}

// Error logs an error message
func (l *TestLogger) Error(msg string, fields ...Field) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, fields)
	}
}

// Fatal logs a fatal message
func (l *TestLogger) Fatal(msg string, fields ...Field) {
	if l.level <= FatalLevel {
		l.log(FatalLevel, msg, fields)
	}
}

// log captures a log entry
func (l *TestLogger) log(level Level, msg string, fields []Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Combine base fields with the provided fields
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	l.entries = append(l.entries, TestEntry{
		Level:   level,
		Message: msg,
		Fields:  allFields,
	})
}

// Debugf logs a formatted debug message
func (l *TestLogger) Debugf(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.logf(DebugLevel, format, args...)
	}
}

// Infof logs a formatted info message
func (l *TestLogger) Infof(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.logf(InfoLevel, format, args...)
	}
}

// Warnf logs a formatted warning message
func (l *TestLogger) Warnf(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.logf(WarnLevel, format, args...)
	}
}

// Errorf logs a formatted error message
func (l *TestLogger) Errorf(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.logf(ErrorLevel, format, args...)
	}
}

// Fatalf logs a formatted fatal message
func (l *TestLogger) Fatalf(format string, args ...interface{}) {
	if l.level <= FatalLevel {
		l.logf(FatalLevel, format, args...)
	}
}

// logf captures a formatted log entry
func (l *TestLogger) logf(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	l.entries = append(l.entries, TestEntry{
		Level:   level,
		Message: msg,
		Fields:  l.fields,
	})
}

// WithField returns a new logger with a field added to the context
func (l *TestLogger) WithField(key string, value interface{}) Logger {
	return l.With(Any(key, value))
}

// WithFields returns a new logger with fields added to the context
func (l *TestLogger) WithFields(fields Fields) Logger {
	newLogger := &TestLogger{
		level:   l.level,
		entries: l.entries,
	}

	// Copy existing fields
	newLogger.fields = make([]Field, len(l.fields))
	copy(newLogger.fields, l.fields)

	// Add new fields
	for k, v := range fields {
		newLogger.fields = append(newLogger.fields, Any(k, v))
	}

	return newLogger
}

// WithError returns a new logger with an error field
func (l *TestLogger) WithError(err error) Logger {
	return l.With(Err(err))
}

// With returns a new logger with the provided fields added to the context
func (l *TestLogger) With(fields ...Field) Logger {
	newLogger := &TestLogger{
		level:   l.level,
		entries: l.entries,
	}

	// Copy existing fields
	newLogger.fields = make([]Field, len(l.fields))
	copy(newLogger.fields, l.fields)

	// Add new fields
	newLogger.fields = append(newLogger.fields, fields...)

	return newLogger
}

// WithContext returns a new logger with context values extracted as fields
func (l *TestLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	fields := ContextExtractor(ctx)
	return l.WithFields(fields)
}

// WithComponent returns a new logger with a component field
func (l *TestLogger) WithComponent(component string) Logger {
	return l.With(Str(ComponentKey, component))
}

// SetLevel sets the minimum log level
func (l *TestLogger) SetLevel(level Level) {
	l.level = level
}

// GetLevel returns the current minimum log level
func (l *TestLogger) GetLevel() Level {
	return l.level
}

// AssertLogged returns true if a log entry with the given level and message was captured
func (l *TestLogger) AssertLogged(level Level, containsMessage string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level == level && strings.Contains(entry.Message, containsMessage) {
			return true
		}
	}

	return false
}

// AssertLoggedWithField returns true if a log entry with the given level, message,
// and field key/value was captured
func (l *TestLogger) AssertLoggedWithField(level Level, containsMessage string, key string, value interface{}) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level != level || !strings.Contains(entry.Message, containsMessage) {
			continue
		}

		// Check fields
		for _, field := range entry.Fields {
			if field.Key == key {
				// For simplicity, just check string representation
				valueStr := fmt.Sprintf("%v", value)
				fieldValueStr := fmt.Sprintf("%v", field.Value)
				if valueStr == fieldValueStr {
					return true
				}
			}
		}
	}

	return false
}
