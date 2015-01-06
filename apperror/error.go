package apperror

import "runtime"

// Defines a custom error type for this application
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

// Implements Error interface
func (e *Error) Error() string {
	return e.Message
}

// Utility function to get stack traces just in case they are needed
func GetStacktrace() string {
	trace := make([]byte, 1024)
	count := runtime.Stack(trace, false)
	return string(trace[:count])
}
