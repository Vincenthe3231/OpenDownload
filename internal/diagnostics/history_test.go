package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRedactsAndExports(t *testing.T) {
	root := t.TempDir()
	history := &History{root: filepath.Join(root, "diagnostics"), enabled: true}
	diagnostic := New(DownloadTransportFailed, StageDownload, true, "Retry download", "diag-1", time.Now()).WithTechnicalDetail(`C:\Users\vince\secret?token=abc`)
	if err := history.Record(diagnostic); err != nil {
		t.Fatal(err)
	}
	entries, err := history.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].TechnicalDetail != "[LOCAL_PATH]" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	export, err := history.Export()
	if err != nil || export == "" {
		t.Fatalf("export failed: %v %q", err, export)
	}
	if _, err := os.Stat(filepath.Join(root, "diagnostics", "diagnostics.log")); err != nil {
		t.Fatal(err)
	}
}
