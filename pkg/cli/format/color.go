package format

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Color codes
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	White      = "\033[37m"
	BoldRed    = "\033[1;31m"
	BoldGreen  = "\033[1;32m"
	BoldYellow = "\033[1;33m"
	BoldBlue   = "\033[1;34m"
	BoldCyan   = "\033[1;36m"
)

var (
	// useColor determines whether to use color in output
	useColor = true
)

// init determines whether colors should be enabled by default
func init() {
	// Disable colors by default on Windows unless using a terminal that supports them
	if runtime.GOOS == "windows" {
		// Check if terminal supports colors (ConEmu, Windows Terminal, etc.)
		// ANSICON is set by ConEmu and other terminals that support ANSI colors
		// WT_SESSION is set by Windows Terminal
		_, hasAnsicon := os.LookupEnv("ANSICON")
		_, hasWT := os.LookupEnv("WT_SESSION")
		useColor = hasAnsicon || hasWT
	}

	// If RUNE_NO_COLOR or NO_COLOR is set, disable colors
	if _, noColor := os.LookupEnv("RUNE_NO_COLOR"); noColor {
		useColor = false
	}
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		useColor = false
	}

	// If output is not a terminal, disable colors (unless forced)
	if _, forceColor := os.LookupEnv("RUNE_FORCE_COLOR"); !forceColor {
		fileInfo, _ := os.Stdout.Stat()
		if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
			useColor = false
		}
	}
}

// EnableColor enables or disables colored output globally
func EnableColor(enable bool) {
	useColor = enable
}

// IsColorEnabled returns whether colored output is enabled
func IsColorEnabled() bool {
	return useColor
}

// Colorize adds color to a string if colors are enabled
func Colorize(color, text string) string {
	if useColor {
		return color + text + Reset
	}
	return text
}

// Success formats a message as a success (green)
func Success(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(Green, msg)
}

// Warning formats a message as a warning (yellow)
func Warning(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(Yellow, msg)
}

// Error formats a message as an error (red)
func Error(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(Red, msg)
}

// Info formats a message as info (cyan)
func Info(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(Cyan, msg)
}

// Highlight formats a message as highlighted (bold cyan)
func Highlight(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(BoldCyan, msg)
}

// StatusSymbol returns a colorized status symbol
func StatusSymbol(success bool) string {
	if success {
		return Colorize(Green, "✓")
	}
	return Colorize(Red, "✗")
}

// Header formats a message as a header (bold blue)
func Header(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(BoldBlue, msg)
}

// Dim formats a message as dimmed (white)
func Dim(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return Colorize(White, msg)
}

// Label formats a key and value with a label style
func Label(key, value string) string {
	return fmt.Sprintf("%s %s", Colorize(BoldCyan, key+":"), value)
}

// StatusLabel formats a status label based on the status value
func StatusLabel(status string) string {
	status = strings.ToLower(status)
	switch status {
	case "running", "success", "succeeded", "healthy", "active", "ready":
		return Colorize(BoldGreen, status)
	case "pending", "waiting", "starting", "initializing":
		return Colorize(BoldYellow, status)
	case "failed", "error", "unhealthy", "terminated":
		return Colorize(BoldRed, status)
	default:
		return Colorize(White, status)
	}
}

// FormatTable colorizes a table of strings based on a header row
func FormatTable(table [][]string, headerRow bool) [][]string {
	result := make([][]string, len(table))
	for i, row := range table {
		result[i] = make([]string, len(row))
		if i == 0 && headerRow {
			// Color the header row
			for j, cell := range row {
				result[i][j] = Colorize(BoldCyan, cell)
			}
		} else {
			// Copy the row unchanged
			copy(result[i], row)
		}
	}
	return result
}
