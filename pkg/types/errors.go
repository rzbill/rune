package types

import (
	"fmt"
)

// ValidationError represents an error that occurs during validation.
type ValidationError struct {
	Message string
}

// Error returns the error message.
func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new ValidationError with the given message.
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		Message: message,
	}
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// WrapValidationError wraps an error with additional context.
func WrapValidationError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf(format, args...)
	if ve, ok := err.(*ValidationError); ok {
		return &ValidationError{
			Message: fmt.Sprintf("%s: %s", message, ve.Message),
		}
	}

	return &ValidationError{
		Message: fmt.Sprintf("%s: %v", message, err),
	}
}
