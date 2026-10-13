// Package apperrors defines application errors that handlers map to HTTP responses.
// The directory is shared/errors; the package name avoids shadowing the standard errors package.
package apperrors

// Code identifies a client-safe error category.
type Code string

const (
	// CodeValidation means the request was rejected before it was applied.
	CodeValidation Code = "validation_error"
	// CodeNotFound means the requested record does not exist.
	CodeNotFound Code = "not_found"
	// CodeConflict means the request conflicts with existing data.
	CodeConflict Code = "conflict"
	// CodeInternal means an unexpected failure occurred.
	CodeInternal Code = "internal_error"
)

// FieldError is one invalid input field.
type FieldError struct {
	Field   string
	Message string
}

// Field builds a field error.
func Field(field, message string) FieldError {
	return FieldError{Field: field, Message: message}
}

// Error is a safe application error. Message is safe to return to clients.
// Err is for logs and must not be copied into an HTTP response.
type Error struct {
	Code    Code
	Message string
	Details []FieldError
	Err     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Validation returns a client input error.
func Validation(message string, details []FieldError) *Error {
	if message == "" {
		message = "request validation failed"
	}
	return &Error{Code: CodeValidation, Message: message, Details: details}
}

// NotFound returns a missing-record error.
func NotFound(message string) *Error {
	return &Error{Code: CodeNotFound, Message: message}
}

// Conflict returns a conflict error.
func Conflict(message string) *Error {
	return &Error{Code: CodeConflict, Message: message}
}

// Internal returns an unexpected error with a client-safe message.
func Internal(message string, err error) *Error {
	if message == "" {
		message = "an unexpected error occurred"
	}
	return &Error{Code: CodeInternal, Message: message, Err: err}
}
