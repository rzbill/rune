package log

import (
	"encoding/json"
	"time"
)

// Field represents a structured log field with a key and value
type Field struct {
	Key   string
	Value interface{}
}

// F creates a log field with the provided key and value
func F(key string, value interface{}) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Err creates an error field
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{
		Key:   "error",
		Value: err.Error(),
	}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Str creates a string field
func Str(key, value string) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Float64 creates a float64 field
func Float64(key string, value float64) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Time creates a time field
func Time(key string, value time.Time) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Any creates a field for any value (alias for F)
func Any(key string, value interface{}) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// JSON creates a field for a JSON-serializable value
func Json(key string, value interface{}) Field {
	// Marshal the value to JSON
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return Field{Key: key, Value: err.Error()}
	}

	return Field{
		Key:   key,
		Value: string(jsonBytes),
	}
}

// Component creates a component field, useful for tagging logs with a component name
func Component(value string) Field {
	return Field{
		Key:   ComponentKey,
		Value: value,
	}
}

// RequestID creates a request ID field
func RequestID(value string) Field {
	return Field{
		Key:   RequestIDKey,
		Value: value,
	}
}

// TraceID creates a trace ID field
func TraceID(value string) Field {
	return Field{
		Key:   TraceIDKey,
		Value: value,
	}
}

// SpanID creates a span ID field
func SpanID(value string) Field {
	return Field{
		Key:   SpanIDKey,
		Value: value,
	}
}
