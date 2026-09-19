// Package download coordinates application level media download jobs.
package download

const (
	defaultWorkerCount      = 8
	defaultConnectionLimit  = 12
	temporaryOutputSuffix   = ".part"
	terminalStatusCompleted = "completed"
	terminalStatusFailed    = "failed"
	terminalStatusCancelled = "cancelled"
)
