package downloader

import "fmt"

// ValidationError describes invalid transfer input or an invalid server response.
type ValidationError struct {
	Subject string
	Detail  string
}

// Error returns a user safe description of invalid transfer data.
func (e *ValidationError) Error() string {
	if e.Subject == "" {
		return e.Detail
	}
	return fmt.Sprintf("invalid %s: %s", e.Subject, e.Detail)
}

// TransportError identifies a failed network operation while preserving its cause.
type TransportError struct {
	Operation string
	Err       error
}

// Error returns the transfer operation and its underlying failure.
func (e *TransportError) Error() string {
	return fmt.Sprintf("%s: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying network or body read failure.
func (e *TransportError) Unwrap() error {
	return e.Err
}

func validationError(subject, detail string) error {
	return &ValidationError{Subject: subject, Detail: detail}
}

func transportError(operation string, err error) error {
	return &TransportError{Operation: operation, Err: err}
}
