package log

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// Implementation of the new Logger interface methods for BaseLogger

// Debug logs a message at the debug level with fields.
func (l *BaseLogger) Debug(msg string, fields ...Field) {
	if l.level <= DebugLevel {
		l.logWithFields(DebugLevel, msg, fields)
	}
}

// Info logs a message at the info level with fields.
func (l *BaseLogger) Info(msg string, fields ...Field) {
	if l.level <= InfoLevel {
		l.logWithFields(InfoLevel, msg, fields)
	}
}

// Warn logs a message at the warn level with fields.
func (l *BaseLogger) Warn(msg string, fields ...Field) {
	if l.level <= WarnLevel {
		l.logWithFields(WarnLevel, msg, fields)
	}
}

// Error logs a message at the error level with fields.
func (l *BaseLogger) Error(msg string, fields ...Field) {
	if l.level <= ErrorLevel {
		l.logWithFields(ErrorLevel, msg, fields)
	}
}

// Fatal logs a message at the fatal level with fields and then exits.
func (l *BaseLogger) Fatal(msg string, fields ...Field) {
	if l.level <= FatalLevel {
		l.logWithFields(FatalLevel, msg, fields)
		os.Exit(1)
	}
}

// Backward compatible methods with key-value pairs

// Debugf logs a message at the debug level with key-value args.
func (l *BaseLogger) Debugf(msg string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, args...)
	}
}

// Infof logs a message at the info level with key-value args.
func (l *BaseLogger) Infof(msg string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, args...)
	}
}

// Warnf logs a message at the warn level with key-value args.
func (l *BaseLogger) Warnf(msg string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, args...)
	}
}

// Errorf logs a message at the error level with key-value args.
func (l *BaseLogger) Errorf(msg string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, args...)
	}
}

// Fatalf logs a message at the fatal level with key-value args and then exits.
func (l *BaseLogger) Fatalf(msg string, args ...interface{}) {
	if l.level <= FatalLevel {
		l.log(FatalLevel, msg, args...)
		os.Exit(1)
	}
}

// WithField returns a new logger with the field added to it.
func (l *BaseLogger) WithField(key string, value interface{}) Logger {
	return l.WithFields(Fields{key: value})
}

// WithFields returns a new logger with the fields added to it.
func (l *BaseLogger) WithFields(fields Fields) Logger {
	newLogger := &BaseLogger{
		level:     l.level,
		formatter: l.formatter,
		outputs:   l.outputs,
		hooks:     l.hooks,
		fields:    Fields{},
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// With adds fields to the logger (new Field-based API)
func (l *BaseLogger) With(fields ...Field) Logger {
	if len(fields) == 0 {
		return l
	}

	newLogger := &BaseLogger{
		level:     l.level,
		formatter: l.formatter,
		outputs:   l.outputs,
		hooks:     l.hooks,
		fields:    Fields{},
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for _, field := range fields {
		newLogger.fields[field.Key] = field.Value
	}

	return newLogger
}

// WithError returns a new logger with the error added as a field.
func (l *BaseLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// WithContext returns a new logger with fields from the context.
func (l *BaseLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}

	fields := ContextExtractor(ctx)
	if len(fields) == 0 {
		return l
	}

	return l.WithFields(fields)
}

// WithComponent returns a new logger with the component field added.
func (l *BaseLogger) WithComponent(component string) Logger {
	return l.WithField(ComponentKey, component)
}

// SetLevel sets the minimum log level.
func (l *BaseLogger) SetLevel(level Level) {
	l.level = level
}

// GetLevel returns the current minimum log level.
func (l *BaseLogger) GetLevel() Level {
	return l.level
}

// Outputs returns the configured log outputs.
func (l *BaseLogger) Outputs() []Output {
	return l.outputs
}

// log creates a log entry from key-value pairs and writes it to all outputs.
func (l *BaseLogger) log(level Level, msg string, args ...interface{}) {
	// Create fields from args (key-value pairs)
	fields := Fields{}
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key, ok := args[i].(string)
			if !ok {
				key = fmt.Sprintf("arg%d", i)
			}
			fields[key] = args[i+1]
		} else {
			fields[fmt.Sprintf("arg%d", i)] = args[i]
		}
	}

	// Merge base fields with entry fields
	entryFields := Fields{}
	for k, v := range l.fields {
		entryFields[k] = v
	}
	for k, v := range fields {
		entryFields[k] = v
	}

	// Create and write the entry
	l.writeEntry(level, msg, entryFields, nil)
}

// logWithFields creates a log entry from Field structs and writes it to all outputs.
func (l *BaseLogger) logWithFields(level Level, msg string, fields []Field) {
	// Convert Field structs to a Fields map
	entryFields := Fields{}

	// Start with logger's fields
	for k, v := range l.fields {
		entryFields[k] = v
	}

	// Add the fields for this log call
	for _, field := range fields {
		entryFields[field.Key] = field.Value
	}

	// Create and write the entry
	l.writeEntry(level, msg, entryFields, nil)
}

// writeEntry creates and writes a log entry with the given fields
func (l *BaseLogger) writeEntry(level Level, msg string, fields Fields, err error) {
	// Get caller info
	_, file, line, ok := runtime.Caller(3) // Adjusted to account for the new method layers
	caller := "unknown"
	if ok {
		parts := strings.Split(file, "/")
		if len(parts) > 2 {
			caller = fmt.Sprintf("%s:%d", strings.Join(parts[len(parts)-2:], "/"), line)
		} else {
			caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	// Create entry
	entry := &Entry{
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Timestamp: time.Now(),
		Caller:    caller,
		Error:     err,
	}

	// Fire hooks
	for _, hook := range l.hooks {
		for _, hookLevel := range hook.Levels() {
			if hookLevel == level {
				if err := hook.Fire(entry); err != nil {
					// If a hook fails, don't stop processing but log the error
					fmt.Fprintf(os.Stderr, "Error firing hook: %v\n", err)
				}
				break
			}
		}
	}

	// Format entry
	formattedEntry, err := l.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting log entry: %v\n", err)
		return
	}

	// Write to all outputs
	for _, output := range l.outputs {
		if err := output.Write(entry, formattedEntry); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to log output: %v\n", err)
		}
	}
}
