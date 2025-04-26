package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config defines logging configuration.
type Config struct {
	// Level sets the minimum log level
	Level string `json:"level" yaml:"level"`

	// Format sets the output format (json, text)
	Format string `json:"format" yaml:"format"`

	// Outputs defines where logs should be written
	Outputs []OutputConfig `json:"outputs" yaml:"outputs"`

	// Sampling defines sampling behavior for high-volume logs
	Sampling *SamplingConfig `json:"sampling" yaml:"sampling"`

	// EnableCaller enables adding caller information to logs
	EnableCaller bool `json:"enable_caller" yaml:"enable_caller"`

	// RedactedFields lists fields that should be redacted (e.g. passwords)
	RedactedFields []string `json:"redacted_fields" yaml:"redacted_fields"`
}

// OutputConfig defines a single log output.
type OutputConfig struct {
	// Type is the output type (console, file, syslog, remote)
	Type string `json:"type" yaml:"type"`

	// Options are type-specific configuration options
	Options map[string]interface{} `json:"options" yaml:"options"`
}

// SamplingConfig defines sampling behavior for high-volume logs.
type SamplingConfig struct {
	// Initial is the initial number of entries to process without sampling
	Initial int `json:"initial" yaml:"initial"`

	// Thereafter is how often to sample after the initial entries
	Thereafter int `json:"thereafter" yaml:"thereafter"`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:  "info",
		Format: "text",
		Outputs: []OutputConfig{
			{
				Type: "console",
				Options: map[string]interface{}{
					"error_to_stderr": true,
				},
			},
		},
		Sampling:     nil, // No sampling by default
		EnableCaller: false,
	}
}

// ApplyConfig creates a logger from a configuration.
func ApplyConfig(config *Config) (Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Parse log level
	level, err := ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Create options
	options := []LoggerOption{
		WithLevel(level),
	}

	// Add formatter
	switch strings.ToLower(config.Format) {
	case "json":
		options = append(options, WithFormatter(&JSONFormatter{
			EnableCaller: config.EnableCaller,
		}))
	case "text", "":
		options = append(options, WithFormatter(&TextFormatter{
			EnableCaller:   config.EnableCaller,
			ShortTimestamp: false,
		}))
	default:
		return nil, fmt.Errorf("invalid log format: %s", config.Format)
	}

	// Create the logger
	logger := NewLogger(options...)

	// Configure outputs
	if len(config.Outputs) == 0 {
		// Add default console output if none specified
		output := NewConsoleOutput(WithErrorToStderr())
		logger.(*BaseLogger).outputs = append(logger.(*BaseLogger).outputs, output)
	} else {
		// Clear default outputs
		logger.(*BaseLogger).outputs = nil

		// Add configured outputs
		for _, outputConfig := range config.Outputs {
			output, err := createOutput(outputConfig)
			if err != nil {
				return nil, err
			}
			logger.(*BaseLogger).outputs = append(logger.(*BaseLogger).outputs, output)
		}
	}

	// Add redaction hook if needed
	if len(config.RedactedFields) > 0 {
		logger.(*BaseLogger).hooks = append(logger.(*BaseLogger).hooks, NewRedactionHook(config.RedactedFields))
	}

	// Add sampling hook if configured
	if config.Sampling != nil && config.Sampling.Thereafter > 0 {
		logger.(*BaseLogger).hooks = append(
			logger.(*BaseLogger).hooks,
			NewSamplingHook(config.Sampling.Initial, config.Sampling.Thereafter),
		)
	}

	return logger, nil
}

// createOutput creates an output from a configuration.
func createOutput(config OutputConfig) (Output, error) {
	switch strings.ToLower(config.Type) {
	case "console":
		options := []ConsoleOutputOption{}

		// Parse options
		if v, ok := config.Options["stderr"].(bool); ok && v {
			options = append(options, WithStderr())
		}
		if v, ok := config.Options["error_to_stderr"].(bool); ok && v {
			options = append(options, WithErrorToStderr())
		}

		return NewConsoleOutput(options...), nil

	case "file":
		// Get filename
		filename, ok := config.Options["filename"].(string)
		if !ok || filename == "" {
			return nil, fmt.Errorf("file output requires a filename")
		}

		// Expand variables and paths
		filename = os.ExpandEnv(filename)
		if !filepath.IsAbs(filename) {
			wd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			filename = filepath.Join(wd, filename)
		}

		options := []FileOutputOption{}

		// Parse max size
		if v, ok := config.Options["max_size"].(string); ok {
			size, err := parseSize(v)
			if err != nil {
				return nil, err
			}
			options = append(options, WithMaxSize(size))
		}

		// Parse max age
		if v, ok := config.Options["max_age"].(string); ok {
			duration, err := time.ParseDuration(v)
			if err != nil {
				return nil, err
			}
			options = append(options, WithMaxAge(duration))
		}

		// Parse max backups
		if v, ok := config.Options["max_backups"].(int); ok {
			options = append(options, WithMaxBackups(v))
		}

		return NewFileOutput(filename, options...), nil

	case "null":
		return NewNullOutput(), nil

	default:
		return nil, fmt.Errorf("unknown output type: %s", config.Type)
	}
}

// ParseLevel parses a level string into a Level.
func ParseLevel(level string) (Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// parseSize parses a size string (e.g., "10MB") into bytes.
func parseSize(size string) (int64, error) {
	size = strings.TrimSpace(size)
	if size == "" {
		return 0, nil
	}

	// Try to parse as a simple number
	var bytes int64
	if _, err := fmt.Sscanf(size, "%d", &bytes); err == nil {
		return bytes, nil
	}

	// Try to parse with units
	var value float64
	var unit string
	if _, err := fmt.Sscanf(size, "%f%s", &value, &unit); err != nil {
		return 0, fmt.Errorf("invalid size format: %s", size)
	}

	unit = strings.ToUpper(unit)
	switch unit {
	case "B":
		return int64(value), nil
	case "KB", "K":
		return int64(value * 1024), nil
	case "MB", "M":
		return int64(value * 1024 * 1024), nil
	case "GB", "G":
		return int64(value * 1024 * 1024 * 1024), nil
	case "TB", "T":
		return int64(value * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}
