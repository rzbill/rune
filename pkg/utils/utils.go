package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

// Helper to safely convert non-negative ints to int32 with clamping
func ToInt32NonNegative(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > int(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(n)
}

func ToInt32NonNegative64(n int64) int32 {
	if n < 0 {
		return 0
	}
	if n > int64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(n)
}

func ToUint32NonNegative(n int) uint32 {
	if n < 0 {
		return 0
	}
	if n > int(math.MaxUint32) {
		return math.MaxUint32
	}
	return uint32(n)
}

func MapToPretty(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func PrettyPrint(v ...interface{}) {
	prints := CapturePrettyPrint(v...)
	fmt.Println(prints...)
}

func CapturePrettyPrint(v ...interface{}) []any {
	prints := []any{}
	for _, value := range v {
		// if v is a string then print it directly
		if str, ok := value.(string); ok {
			prints = append(prints, str)
			continue
		}

		if str, ok := value.(*string); ok {
			prints = append(prints, *str)
			continue
		}

		if intValue, ok := value.(int); ok {
			prints = append(prints, strconv.Itoa(intValue))
			continue
		}

		if intValue, ok := value.(*int); ok {
			prints = append(prints, strconv.Itoa(*intValue))
			continue
		}

		if intValue, ok := value.(int64); ok {
			prints = append(prints, strconv.FormatInt(intValue, 10))
			continue
		}

		if intValue, ok := value.(*int64); ok {
			prints = append(prints, strconv.FormatInt(*intValue, 10))
			continue
		}

		if floatValue, ok := value.(float64); ok {
			prints = append(prints, strconv.FormatFloat(floatValue, 'f', -1, 64))
			continue
		}

		if floatValue, ok := value.(*float64); ok {
			prints = append(prints, strconv.FormatFloat(*floatValue, 'f', -1, 64))
			continue
		}

		// default to pretty print
		prints = append(prints, MapToPretty(value))
	}

	return prints
}

func ProtoStringToTimestamp(timestamp string) *time.Time {
	if timestamp == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil
	}
	return &parsed
}

// ParseTimestamp parses a timestamp string into a time.Time.
func ParseTimestamp(timestampStr string) (*time.Time, error) {
	// Parse created_at timestamp
	if timestampStr != "" {
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		return &timestamp, nil
	}
	return nil, nil
}
