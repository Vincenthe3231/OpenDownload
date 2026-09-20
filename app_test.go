package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/nativehost"
)

func TestManualCaptureCompatibilityAPI(t *testing.T) {
	app := NewApp()
	if _, err := app.StartManualCapture(); err != nil {
		t.Fatalf("StartManualCapture returned error: %v", err)
	}
	app.StopBrowserCapture()

	if _, err := app.StartFirefoxCapture(); err != nil {
		t.Fatalf("StartFirefoxCapture compatibility wrapper returned error: %v", err)
	}
	app.StopBrowserCapture()
}

func TestDeveloperDiagnosticsToggleControlsTechnicalStore(t *testing.T) {
	app := NewApp()
	if app.DeveloperDiagnosticsEnabled() {
		t.Fatal("developer diagnostics enabled by default")
	}
	if err := app.SetDeveloperDiagnostics(true); err != nil {
		t.Fatalf("enable developer diagnostics: %v", err)
	}
	if !app.DeveloperDiagnosticsEnabled() || !app.capture.DeveloperDiagnosticsEnabled() {
		t.Fatal("developer diagnostics did not enable the technical store")
	}
	if err := app.SetDeveloperDiagnostics(false); err != nil {
		t.Fatalf("disable developer diagnostics: %v", err)
	}
	if app.DeveloperDiagnosticsEnabled() || app.capture.DeveloperDiagnosticsEnabled() {
		t.Fatal("developer diagnostics did not disable the technical store")
	}
}

func TestDownloadTechnicalContextUsesOpaqueDiagnosticIDAndClears(t *testing.T) {
	app := NewApp()
	const diagnosticID = "download-diagnostic-id"

	if got := app.GetDownloadTechnicalContext(diagnosticID); got != nil {
		t.Fatal("download technical context was available while diagnostics were disabled")
	}
	if got := app.GetDownloadTechnicalContext(" "); got != nil {
		t.Fatal("blank diagnostic ID returned technical context")
	}

	if err := app.SetDeveloperDiagnostics(true); err != nil {
		t.Fatalf("enable developer diagnostics: %v", err)
	}
	app.technicalStore.Record(diagnostics.TechnicalDiagnostic{
		DiagnosticID:   diagnosticID,
		RequestURL:     "https://example.test/video.mp4",
		RawError:       "request failed",
		RequestHeaders: map[string]string{"Cookie": "session-secret"},
	})

	got := app.GetDownloadTechnicalContext(diagnosticID)
	if got == nil || got.RequestURL != "https://example.test/video.mp4" || got.RequestHeaders["Cookie"] != "session-secret" {
		t.Fatalf("unexpected download technical context: %#v", got)
	}

	if err := app.SetDeveloperDiagnostics(false); err != nil {
		t.Fatalf("disable developer diagnostics: %v", err)
	}
	if got := app.GetDownloadTechnicalContext(diagnosticID); got != nil {
		t.Fatal("download technical context survived diagnostics disable")
	}
}

func TestStartupRecordsSafeNativeServerFailure(t *testing.T) {
	app := NewApp()
	app.startNativeServer = func(context.Context) (func(), error) {
		return nil, errors.New("named pipe unavailable")
	}

	_, _ = app.startNativeCapture(context.Background())
	entries := app.ListDiagnosticHistory()
	if len(entries) == 0 {
		t.Fatal("startup failure did not publish a diagnostic")
	}
	diagnostic := entries[len(entries)-1]
	if diagnostic.Code != diagnostics.IPCAccessDenied {
		t.Fatalf("diagnostic code = %q, want %q", diagnostic.Code, diagnostics.IPCAccessDenied)
	}
	if diagnostic.UserMessage != "Automatic capture is unavailable. Repair browser capture." {
		t.Fatalf("diagnostic message exposed unexpected detail: %q", diagnostic.UserMessage)
	}
	app.shutdown(context.Background())
}

func TestAutomaticCaptureStatusAndRepair(t *testing.T) {
	var actions []string
	runner := nativehost.NewRegistrationRunnerWithOptions(`C:\OpenDownload`, func(_ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		actions = append(actions, joined)
		return []byte(`{"hostExecutableFound":true,"manifestExists":true,"manifestPathMatchesInstall":true,"manifestExtensionIdMatches":true,"mozillaRegistryPointsToExpectedManifest":true,"healthy":true}`), nil
	})
	app := NewApp()
	app.registration = runner

	status := app.GetAutomaticCaptureStatus()
	if !status.Healthy || status.Failure != nil {
		t.Fatalf("unexpected automatic capture status: %+v", status)
	}
	if err := app.RepairAutomaticCapture(); err != nil {
		t.Fatalf("RepairAutomaticCapture returned error: %v", err)
	}
	if len(actions) != 2 || !strings.Contains(actions[0], "Inspect") || !strings.Contains(actions[1], "Install") {
		t.Fatalf("unexpected registration actions: %v", actions)
	}
}

func TestNewAppExposesFreshServices(t *testing.T) {
	app := NewApp()
	if app.downloads == nil || app.capture == nil {
		t.Fatal("NewApp must initialize the application services")
	}
}
