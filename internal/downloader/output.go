package downloader

import (
	"fmt"
	"os"
)

// createWorkOutput creates the caller reserved work file exactly at path.
// Downloaders never publish this file to a final destination.
func createWorkOutput(path string) (*os.File, error) {
	if path == "" {
		return nil, validationError("work output path", "must not be empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create work output: %w", err)
	}
	return file, nil
}

func removeWorkOutput(path string) {
	_ = os.Remove(path)
}
