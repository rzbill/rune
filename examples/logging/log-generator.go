package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// LogLevel represents different log severity levels
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

// LogMessage represents a structured log message
type LogMessage struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
}

// Possible service components
var components = []string{
	"api", "database", "cache", "worker", "scheduler", "auth", "notification",
}

// Possible example messages
var messages = map[LogLevel][]string{
	DEBUG: {
		"Processing request payload",
		"Cache hit for key: %s",
		"Database query executed in %dms",
		"Connection pool stats: active=%d idle=%d",
		"Rate limiter current rate: %d requests/second",
	},
	INFO: {
		"Request processed successfully",
		"User %s logged in",
		"New item created with id: %s",
		"Scheduled task started: %s",
		"API response sent with status code: %d",
	},
	WARN: {
		"High memory usage detected: %d%%",
		"API request took longer than expected: %dms",
		"Cache miss for frequently accessed key: %s",
		"Using fallback method for: %s",
		"Retrying operation, attempt %d of 3",
	},
	ERROR: {
		"Failed to connect to database: %s",
		"API request failed: %s",
		"Unable to process message: %s",
		"Validation error: %s",
		"Operation timed out after %dms",
	},
	FATAL: {
		"Unrecoverable system error: %s",
		"Critical resource unavailable: %s",
		"Security breach detected: %s",
		"Data corruption detected in: %s",
		"Unable to start required service: %s",
	},
}

// Random user IDs
var userIDs = []string{
	"user-123", "user-456", "user-789", "admin-001", "guest-555",
}

// Random operation IDs
var operationIDs = []string{
	"op-abc123", "op-def456", "op-ghi789", "op-jkl012", "op-mno345",
}

// Random error strings
var errorStrings = []string{
	"connection refused", "timeout", "permission denied", "not found", "invalid input",
}

func main() {
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Determine log interval from environment or use default
	interval := 1000 // milliseconds
	if envInterval := os.Getenv("LOG_INTERVAL"); envInterval != "" {
		fmt.Sscanf(envInterval, "%d", &interval)
	}

	fmt.Printf("Log generator started. Producing logs every %d milliseconds...\n", interval)
	fmt.Println("Press Ctrl+C to stop.")

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()

	// Seed the random generator
	rand.Seed(time.Now().UnixNano())

	for {
		select {
		case <-ticker.C:
			// Generate and print a random log message
			log := generateRandomLog()
			printLog(log)
		case <-sigChan:
			fmt.Println("Shutting down log generator...")
			return
		}
	}
}

// generateRandomLog creates a random log message
func generateRandomLog() LogMessage {
	// Randomly select log level with weighted distribution
	level := randomLogLevel()

	// Select a random component
	component := components[rand.Intn(len(components))]

	// Select a random message template for this level
	msgTemplates := messages[level]
	template := msgTemplates[rand.Intn(len(msgTemplates))]

	// Create random field values
	fields := map[string]interface{}{
		"component":    component,
		"request_id":   fmt.Sprintf("req-%06x", rand.Intn(0xffffff)),
		"elapsed_time": rand.Intn(2000),
	}

	// Format the message with random values
	msg := formatMessage(template, level)

	return LogMessage{
		Level:   level,
		Message: msg,
		Fields:  fields,
	}
}

// randomLogLevel returns a random log level with weighted distribution
func randomLogLevel() LogLevel {
	n := rand.Intn(100)
	switch {
	case n < 40:
		return INFO
	case n < 70:
		return DEBUG
	case n < 85:
		return WARN
	case n < 98:
		return ERROR
	default:
		return FATAL
	}
}

// formatMessage replaces placeholders in the message template with random values
func formatMessage(template string, level LogLevel) string {
	msg := template

	if strings.Contains(msg, "%s") {
		switch level {
		case DEBUG, INFO:
			if rand.Intn(2) == 0 {
				msg = fmt.Sprintf(msg, userIDs[rand.Intn(len(userIDs))])
			} else {
				msg = fmt.Sprintf(msg, operationIDs[rand.Intn(len(operationIDs))])
			}
		case WARN, ERROR, FATAL:
			msg = fmt.Sprintf(msg, errorStrings[rand.Intn(len(errorStrings))])
		}
	} else if strings.Contains(msg, "%d") {
		var value int
		if strings.Contains(msg, "%%") {
			value = rand.Intn(100)
		} else if strings.Contains(msg, "ms") {
			value = rand.Intn(1000)
		} else {
			value = rand.Intn(500)
		}
		msg = fmt.Sprintf(msg, value)
	}

	return msg
}

// printLog formats and prints a log message
func printLog(log LogMessage) {
	// Format timestamp: 2023-05-03T12:34:56.789Z
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Format fields
	var fieldsStr string
	if len(log.Fields) > 0 {
		var parts []string
		for k, v := range log.Fields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		fieldsStr = " " + strings.Join(parts, " ")
	}

	// Different colors for different log levels
	var colorCode, resetCode string
	if isatty() {
		resetCode = "\033[0m"
		switch log.Level {
		case DEBUG:
			colorCode = "\033[37m" // White
		case INFO:
			colorCode = "\033[32m" // Green
		case WARN:
			colorCode = "\033[33m" // Yellow
		case ERROR:
			colorCode = "\033[31m" // Red
		case FATAL:
			colorCode = "\033[35m" // Magenta
		}
	}

	fmt.Printf("%s%s [%s] %s%s%s\n",
		colorCode, timestamp, log.Level, log.Message, fieldsStr, resetCode)
}

// isatty returns true if stdout is a terminal
func isatty() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
