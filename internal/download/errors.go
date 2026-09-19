package download

import "fmt"

// ConflictError identifies a job ID or output path that cannot be reserved.
type ConflictError struct {
	Resource string
	Value    string
}

// Error implements error.
func (err *ConflictError) Error() string {
	return fmt.Sprintf("%s is already in use: %s", err.Resource, err.Value)
}
