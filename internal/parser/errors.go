package parser

import "fmt"

// ValidationError describes malformed or unsupported manifest data.
type ValidationError struct {
	Subject string
	Detail  string
}

// Error returns a user safe description of invalid manifest data.
func (e *ValidationError) Error() string {
	if e.Subject == "" {
		return e.Detail
	}
	return fmt.Sprintf("invalid %s: %s", e.Subject, e.Detail)
}

func validationError(subject, detail string) error {
	return &ValidationError{Subject: subject, Detail: detail}
}
