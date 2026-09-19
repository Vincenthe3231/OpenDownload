package download

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func reserveOutputPath(directory, filename, jobID string) (string, string, error) {
	if strings.TrimSpace(directory) == "" {
		return "", "", fmt.Errorf("output directory is required")
	}
	if strings.TrimSpace(filename) == "" {
		return "", "", fmt.Errorf("output filename is required")
	}
	absDirectory, err := filepath.Abs(directory)
	if err != nil {
		return "", "", fmt.Errorf("resolve output directory: %w", err)
	}
	finalPath := filepath.Clean(filepath.Join(absDirectory, filename))
	if filepath.Dir(finalPath) != absDirectory {
		return "", "", fmt.Errorf("output filename escapes the selected directory")
	}
	workPath := finalPath + temporaryOutputSuffix + "." + safePathPart(jobID)
	return finalPath, workPath, nil
}

func safePathPart(value string) string {
	var builder strings.Builder
	for _, runeValue := range value {
		switch {
		case runeValue >= 'a' && runeValue <= 'z':
			builder.WriteRune(runeValue)
		case runeValue >= 'A' && runeValue <= 'Z':
			builder.WriteRune(runeValue)
		case runeValue >= '0' && runeValue <= '9':
			builder.WriteRune(runeValue)
		default:
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "job"
	}
	return builder.String()
}

func publishOutput(workPath, finalPath, jobID string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Lstat(finalPath); err == nil {
			return &ConflictError{Resource: "output path", Value: finalPath}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect output path: %w", err)
		}
		if err := os.Rename(workPath, finalPath); err != nil {
			return fmt.Errorf("publish output: %w", err)
		}
		return nil
	}

	if _, err := os.Lstat(finalPath); os.IsNotExist(err) {
		if renameErr := os.Rename(workPath, finalPath); renameErr != nil {
			return fmt.Errorf("publish output: %w", renameErr)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect output path: %w", err)
	}

	backupPath := finalPath + ".previous." + safePathPart(jobID)
	if _, err := os.Lstat(backupPath); err == nil {
		return &ConflictError{Resource: "output backup path", Value: backupPath}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output backup path: %w", err)
	}
	if err := os.Rename(finalPath, backupPath); err != nil {
		return fmt.Errorf("stage existing output for replacement: %w", err)
	}
	if err := os.Rename(workPath, finalPath); err != nil {
		if restoreErr := os.Rename(backupPath, finalPath); restoreErr != nil {
			return fmt.Errorf("publish output: %w; restore previous output: %v", err, restoreErr)
		}
		return fmt.Errorf("publish output: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("remove replaced output backup: %w", err)
	}
	return nil
}
