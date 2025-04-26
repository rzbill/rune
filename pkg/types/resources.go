package types

import (
	"strconv"
	"strings"
)

// ResourceHelpers contains utility functions for working with Resources

// ParseCPU parses a CPU request/limit string into a float64 (cores)
// Examples: "100m" -> 0.1, "0.5" -> 0.5, "2" -> 2.0
func ParseCPU(cpu string) (float64, error) {
	if cpu == "" {
		return 0, nil
	}

	// Check for millicores (e.g. "100m")
	if strings.HasSuffix(cpu, "m") {
		millicores, err := strconv.Atoi(cpu[:len(cpu)-1])
		if err != nil {
			return 0, err
		}
		return float64(millicores) / 1000.0, nil
	}

	// Regular floating point value
	return strconv.ParseFloat(cpu, 64)
}

// ParseMemory parses a memory request/limit string into bytes
// Examples: "100Mi" -> 104857600, "1Gi" -> 1073741824, "2G" -> 2000000000
func ParseMemory(memory string) (int64, error) {
	if memory == "" {
		return 0, nil
	}

	// Units and their byte equivalents
	units := map[string]int64{
		"":   1,
		"k":  1000,
		"m":  1000000,
		"g":  1000000000,
		"t":  1000000000000,
		"p":  1000000000000000,
		"e":  1000000000000000000,
		"ki": 1024,
		"mi": 1048576,
		"gi": 1073741824,
		"ti": 1099511627776,
		"pi": 1125899906842624,
		"ei": 1152921504606846976,
	}

	memory = strings.ToLower(memory)
	var unit string
	var value string

	// Find the unit suffix
	for i := len(memory) - 1; i >= 0; i-- {
		if strings.ContainsAny(string(memory[i]), "0123456789.") {
			value = memory[:i+1]
			unit = strings.ToLower(memory[i+1:])
			break
		}
	}

	// If no unit found, the entire string is the value
	if value == "" {
		value = memory
		unit = ""
	}

	// Parse the numeric part
	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}

	// Find the multiplier for the unit
	multiplier, ok := units[unit]
	if !ok {
		return 0, NewValidationError("unknown memory unit: " + unit)
	}

	return int64(num * float64(multiplier)), nil
}

// FormatCPU formats a CPU value as a string
// Examples: 0.1 -> "100m", 0.5 -> "500m", 2.0 -> "2"
func FormatCPU(cpu float64) string {
	if cpu == 0 {
		return "0"
	}

	// If it's a whole number or close to it, format as an integer
	if cpu >= 1.0 && cpu == float64(int64(cpu)) {
		return strconv.FormatInt(int64(cpu), 10)
	}

	// Otherwise, format as millicores
	return strconv.FormatInt(int64(cpu*1000), 10) + "m"
}

// FormatMemory formats memory bytes as a human-readable string
// Uses binary units (Ki, Mi, Gi, etc.)
func FormatMemory(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + "B"
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + units[exp]
}
