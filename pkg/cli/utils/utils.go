package utils

import (
	"fmt"
	"strings"
)

// ParseSelector parses a selector string into a map of key-value pairs
// Example: "app=frontend,env=prod" -> {"app": "frontend", "env": "prod"}
func ParseSelector(selector string) (map[string]string, error) {
	result := make(map[string]string)
	if selector == "" {
		return result, nil
	}

	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid selector format, expected key=value: %s", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty key in selector: %s", pair)
		}
		if value == "" {
			return nil, fmt.Errorf("empty value in selector: %s", pair)
		}
		if strings.Contains(value, "=") {
			return nil, fmt.Errorf("invalid value format, contains additional equals sign: %s", value)
		}
		result[key] = value
	}

	return result, nil
}
