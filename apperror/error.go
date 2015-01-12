// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apperror

import "runtime"

// Error defines a custom error type for this application.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

// Implements Error interface.
func (e *Error) Error() string {
	return e.Message
}

// Utility function to get stack traces just in case they are needed.
func GetStacktrace() string {
	trace := make([]byte, 1024)
	count := runtime.Stack(trace, false)
	return string(trace[:count])
}
