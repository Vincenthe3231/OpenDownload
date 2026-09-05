package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDownloadDirUsesUserDownloads(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	dir, err := defaultDownloadDir()
	if err != nil {
		t.Fatalf("defaultDownloadDir: %v", err)
	}
	if want := filepath.Join(home, "Downloads"); dir != want {
		t.Fatalf("defaultDownloadDir = %q, want %q", dir, want)
	}
}
