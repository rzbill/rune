package log

import (
	"io"
	stdlog "log"
)

// ToStdLogger converts our structured Logger to a standard library *log.Logger
// This is helpful for integrating with libraries or components that expect a standard logger
func ToStdLogger(logger Logger, prefix string, flag int) *stdlog.Logger {
	if flag == 0 {
		flag = stdlog.LstdFlags
	}
	return stdlog.New(&logAdapter{logger: logger}, prefix, flag)
}

// logAdapter adapts our Logger to io.Writer for use with standard log package
type logAdapter struct {
	logger Logger
}

// Write implements io.Writer, converting written content to structured log messages
func (a *logAdapter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	// Log at info level
	a.logger.Info(msg)

	return len(p), nil
}

// StdLogWriter creates an io.Writer that logs lines at the specified level
// Useful for redirecting output from third-party libraries
func StdLogWriter(logger Logger, level Level) io.Writer {
	return &leveledLogAdapter{
		logger: logger,
		level:  level,
	}
}

// leveledLogAdapter adapts our Logger to io.Writer with level control
type leveledLogAdapter struct {
	logger Logger
	level  Level
}

// Write implements io.Writer, writing to the logger at the configured level
func (a *leveledLogAdapter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	switch a.level {
	case DebugLevel:
		a.logger.Debug(msg)
	case InfoLevel:
		a.logger.Info(msg)
	case WarnLevel:
		a.logger.Warn(msg)
	case ErrorLevel:
		a.logger.Error(msg)
	case FatalLevel:
		a.logger.Error(msg) // Don't use Fatal to avoid os.Exit
	default:
		a.logger.Info(msg)
	}

	return len(p), nil
}

// RedirectStdLog redirects the standard library's default logger to our logger
// This is useful for capturing logs from third-party libraries that use the
// standard log package directly
func RedirectStdLog(logger Logger) func() {
	// Save the original settings
	origOutput := stdlog.Writer()
	origFlags := stdlog.Flags()
	origPrefix := stdlog.Prefix()

	// Redirect to our logger
	stdlog.SetOutput(StdLogWriter(logger, InfoLevel))
	stdlog.SetFlags(0) // Remove standard logger's built-in prefixes

	// Return a function that restores the original settings
	return func() {
		stdlog.SetOutput(origOutput)
		stdlog.SetFlags(origFlags)
		stdlog.SetPrefix(origPrefix)
	}
}
