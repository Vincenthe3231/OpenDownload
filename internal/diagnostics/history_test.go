package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryNeverPersistsTechnicalDetail(t *testing.T) {
	root := t.TempDir()
	history := &History{root: filepath.Join(root, "diagnostics"), enabled: true}
	diagnostic := New(DownloadTransportFailed, StageDownload, true, "Retry download", "diag-1", time.Now())
	diagnostic = diagnostic.WithTechnicalDetail(`C:\Users\vince\secret?token=abc`)
	if err := history.Record(diagnostic); err != nil {
		t.Fatal(err)
	}
	entries, err := history.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	export, err := history.Export()
	if err != nil || export == "" {
		t.Fatalf("export failed: %v %q", err, export)
	}
	path := filepath.Join(root, "diagnostics", "diagnostics.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret?token=abc") || strings.Contains(string(data), "technicalDetail") || strings.Contains(export, "secret?token=abc") || strings.Contains(export, "technicalDetail") {
		t.Fatalf("history export retained technical data: %s", export)
	}
}

func TestNewHistoryDoesNotRestoreDeveloperDiagnostics(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	settingsPath := filepath.Join(root, "OpenDownload", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"developerDiagnostics":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if history := NewHistory(); history.Enabled() {
		t.Fatal("NewHistory restored developer diagnostics from disk")
	}
}
