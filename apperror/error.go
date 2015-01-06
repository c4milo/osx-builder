package apperror

import "runtime"

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

func GetStacktrace() string {
	trace := make([]byte, 1024)
	count := runtime.Stack(trace, false)
	return string(trace[:count])
}
