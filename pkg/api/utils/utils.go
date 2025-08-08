package utils

import "time"

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
