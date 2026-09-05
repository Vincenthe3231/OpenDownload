package transport

import "fmt"

// HTTPStatusError reports an HTTP response that was outside the expected
// success range.
type HTTPStatusError struct {
	StatusCode int
	Status     string
}

// Error implements error.
func (err *HTTPStatusError) Error() string {
	if err.Status == "" {
		return fmt.Sprintf("HTTP %d", err.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", err.StatusCode, err.Status)
}
