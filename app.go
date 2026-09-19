package main

import (
	"context"
	"time"

	"github.com/opendownload/opendownload/internal/capture"
	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/download"
	"github.com/opendownload/opendownload/internal/nativehost"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DownloadRequest is the public desktop request for a direct media download.
type DownloadRequest struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	OutputDir string `json:"outputDir"`
}

// CapturedDownloadRequest is the public desktop request for a paired capture.
type CapturedDownloadRequest struct {
	ID               string `json:"id"`
	CapturedStreamID string `json:"capturedStreamId"`
	OutputDir        string `json:"outputDir"`
}

type nativeServerStarter func(context.Context) (func(), error)

// AutomaticCaptureFailure is the safe failure returned by the Gecko
// registration inspection and repair APIs.
type AutomaticCaptureFailure struct {
	Code        string `json:"code"`
	UserMessage string `json:"userMessage"`
	Retryable   bool   `json:"retryable"`
}

// AutomaticCaptureStatus describes the installed Gecko native-host wiring.
type AutomaticCaptureStatus struct {
	HostExecutableFound             bool                     `json:"hostExecutableFound"`
	ManifestExists                  bool                     `json:"manifestExists"`
	ManifestPathMatchesInstall      bool                     `json:"manifestPathMatchesInstall"`
	ManifestExtensionIDMatches      bool                     `json:"manifestExtensionIdMatches"`
	MozillaRegistryPointsToManifest bool                     `json:"mozillaRegistryPointsToExpectedManifest"`
	Healthy                         bool                     `json:"healthy"`
	Failure                         *AutomaticCaptureFailure `json:"failure,omitempty"`
}

// App is the thin Wails adapter around local application services.
type App struct {
	ctx                context.Context
	capture            *capture.Manager
	downloads          *download.Service
	stopDownloadEvents func()
	stopCaptureEvents  func()
	stopNativeCapture  func()
	diagnosticHistory  *diagnostics.History
	technicalStore     *diagnostics.TechnicalStore
	registration       *nativehost.RegistrationRunner
	startNativeServer  nativeServerStarter
}

// NewApp creates the desktop application adapter.
func NewApp() *App {
	technicalStore := diagnostics.NewTechnicalStore()
	captureManager := capture.NewManager()
	captureManager.SetTechnicalStore(technicalStore)
	app := &App{
		capture:           captureManager,
		downloads:         download.NewService(download.Config{}),
		diagnosticHistory: diagnostics.NewHistory(),
		technicalStore:    technicalStore,
		registration:      nativehost.NewRegistrationRunner(),
		startNativeServer: captureManager.StartNativeServer,
	}
	captureManager.SetDiagnosticReporter(app.setDiagnostic)
	return app
}

func (app *App) startup(ctx context.Context) {
	app.ctx = ctx
	app.stopDownloadEvents = app.downloads.Subscribe(func(snapshot download.JobSnapshot) {
		runtime.EventsEmit(ctx, downloadChangedEvent, snapshot)
	})
	app.stopCaptureEvents = app.capture.Subscribe(func(event capture.Event) {
		switch event.Type {
		case capture.EventSessionChanged:
			runtime.EventsEmit(ctx, string(event.Type), event.Session)
		case capture.EventStreamAdded:
			runtime.EventsEmit(ctx, string(event.Type), event.Stream)
		}
	})
	_, _ = app.startNativeCapture(ctx)
}

func (app *App) startNativeCapture(ctx context.Context) (func(), error) {
	starter := app.startNativeServer
	if starter == nil {
		starter = app.capture.StartNativeServer
	}
	stopNativeCapture, err := starter(ctx)
	app.stopNativeCapture = stopNativeCapture
	if err != nil {
		app.setDiagnostic(diagnostics.New(
			diagnostics.IPCAccessDenied,
			diagnostics.StageNativeConnection,
			true,
			"Automatic capture is unavailable. Repair browser capture.",
			diagnostics.NewID(),
			time.Now(),
		))
	}
	return stopNativeCapture, err
}

func (app *App) shutdown(context.Context) {
	if app.stopDownloadEvents != nil {
		app.stopDownloadEvents()
	}
	if app.stopCaptureEvents != nil {
		app.stopCaptureEvents()
	}
	app.capture.ClearTechnicalCapture()
	if app.technicalStore != nil {
		app.technicalStore.SetEnabled(false)
	}
	if app.stopNativeCapture != nil {
		app.stopNativeCapture()
	}
	app.downloads.CancelAll()
	app.capture.Stop()
}

// QueueDownload schedules a direct URL download and returns its initial snapshot.
func (app *App) QueueDownload(request DownloadRequest) (download.JobSnapshot, error) {
	return app.downloads.Queue(app.context(), download.Request{
		ID:        request.ID,
		URL:       request.URL,
		OutputDir: request.OutputDir,
	})
}

// CancelDownload requests cancellation and returns the latest job snapshot.
func (app *App) CancelDownload(jobID string) (download.JobSnapshot, error) {
	return app.downloads.Cancel(jobID)
}

// ListDownloadJobs returns snapshots for client hydration.
func (app *App) ListDownloadJobs() []download.JobSnapshot {
	return app.downloads.List()
}

// StartManualCapture starts a fresh paired browser capture session.
func (app *App) StartManualCapture() (capture.Pairing, error) {
	pairing, err := app.capture.Start()
	if err != nil {
		app.setDiagnostic(diagnostics.New(diagnostics.CaptureRequestRejected, diagnostics.StageCapture, true, "Could not start browser capture.", diagnostics.NewID(), time.Now()).WithTechnicalDetail(err.Error()))
	}
	return pairing, err
}

// StartFirefoxCapture preserves the original Wails API for existing clients.
func (app *App) StartFirefoxCapture() (capture.Pairing, error) {
	return app.StartManualCapture()
}

// StopFirefoxCapture clears the active paired browser capture session.
func (app *App) StopFirefoxCapture() {
	app.capture.Stop()
}

// GetCaptureSession returns safe state for automatic and manual capture UI.
func (app *App) GetCaptureSession() capture.SessionSnapshot {
	return app.capture.Session()
}

// StopBrowserCapture stops whichever capture mode is active.
func (app *App) StopBrowserCapture() {
	app.capture.Stop()
}

// SetCaptureDiagnostic stores and publishes a redacted diagnostic.
func (app *App) SetCaptureDiagnostic(diagnostic diagnostics.Diagnostic) error {
	app.setDiagnostic(diagnostic)
	return nil
}

// DeveloperDiagnosticsEnabled returns the runtime-only technical diagnostics state.
func (app *App) DeveloperDiagnosticsEnabled() bool {
	return app.technicalStore != nil && app.technicalStore.Enabled()
}

// SetDeveloperDiagnostics controls technical detail persistence and display.
func (app *App) SetDeveloperDiagnostics(enabled bool) error {
	if app.technicalStore != nil {
		app.technicalStore.SetEnabled(enabled)
	}
	if !enabled {
		app.capture.ClearTechnicalCapture()
	}
	return app.diagnosticHistory.SetEnabled(enabled)
}

// GetTechnicalContext returns sensitive capture detail only while the
// runtime-only developer diagnostic mode is enabled.
func (app *App) GetTechnicalContext(streamID string) *diagnostics.TechnicalDiagnostic {
	if !app.DeveloperDiagnosticsEnabled() {
		return nil
	}
	return app.capture.TechnicalContext(streamID)
}

// GetAutomaticCaptureStatus inspects the installed Gecko native-host wiring.
func (app *App) GetAutomaticCaptureStatus() AutomaticCaptureStatus {
	runner := app.registration
	if runner == nil {
		runner = nativehost.NewRegistrationRunner()
	}
	status, err := runner.Inspect()
	if err != nil {
		diagnostic := diagnostics.New(
			diagnostics.NativeHostNotRegistered,
			diagnostics.StageNativeConnection,
			true,
			"Firefox browser capture registration could not be inspected. Repair browser capture.",
			diagnostics.NewID(),
			time.Now(),
		)
		app.setDiagnostic(diagnostic)
		return AutomaticCaptureStatus{
			Healthy: false,
			Failure: &AutomaticCaptureFailure{
				Code:        string(diagnostic.Code),
				UserMessage: diagnostic.UserMessage,
				Retryable:   diagnostic.Retryable,
			},
		}
	}
	return automaticCaptureStatus(status)
}

// RepairAutomaticCapture re-runs the bundled Gecko native-host registration.
func (app *App) RepairAutomaticCapture() error {
	runner := app.registration
	if runner == nil {
		runner = nativehost.NewRegistrationRunner()
	}
	if err := runner.Install(); err != nil {
		diagnostic := diagnostics.New(
			diagnostics.NativeHostNotRegistered,
			diagnostics.StageNativeConnection,
			true,
			"Firefox browser capture repair failed. Check the installer, then try again.",
			diagnostics.NewID(),
			time.Now(),
		)
		app.setDiagnostic(diagnostic)
		return diagnostics.NewError(diagnostic)
	}
	return nil
}

// ListDiagnosticHistory returns recent redacted diagnostics.
func (app *App) ListDiagnosticHistory() []diagnostics.Diagnostic {
	entries, err := app.diagnosticHistory.List()
	if err != nil {
		return nil
	}
	return entries
}

// ExportDiagnosticHistory returns recent redacted diagnostics as JSON.
func (app *App) ExportDiagnosticHistory() (string, error) {
	return app.diagnosticHistory.Export()
}

func (app *App) setDiagnostic(diagnostic diagnostics.Diagnostic) {
	_ = app.diagnosticHistory.Record(diagnostic)
	app.capture.SetDiagnostic(diagnostic)
}

func automaticCaptureStatus(status nativehost.RegistrationStatus) AutomaticCaptureStatus {
	result := AutomaticCaptureStatus{
		HostExecutableFound:             status.HostExecutableFound,
		ManifestExists:                  status.ManifestExists,
		ManifestPathMatchesInstall:      status.ManifestPathMatchesInstall,
		ManifestExtensionIDMatches:      status.ManifestExtensionIDMatches,
		MozillaRegistryPointsToManifest: status.MozillaRegistryPointsToManifest,
		Healthy:                         status.Healthy,
	}
	if status.Failure != nil {
		result.Failure = &AutomaticCaptureFailure{
			Code:        status.Failure.Code,
			UserMessage: status.Failure.UserMessage,
			Retryable:   status.Failure.Retryable,
		}
	}
	return result
}

// ListCapturedStreams returns public stream summaries without URLs or request headers.
func (app *App) ListCapturedStreams() []capture.StreamSummary {
	return app.capture.List()
}

// QueueCapturedStream schedules a download using request headers kept in memory by capture.
func (app *App) QueueCapturedStream(request CapturedDownloadRequest) (download.JobSnapshot, error) {
	stream, headers, ok := app.capture.Get(request.CapturedStreamID)
	if !ok {
		diagnostic := diagnostics.New(diagnostics.CaptureRequestRejected, diagnostics.StageCapture, true, "Captured stream is no longer available.", diagnostics.NewID(), time.Now()).WithContext(map[string]string{"operationId": request.ID})
		app.setDiagnostic(diagnostic)
		return download.JobSnapshot{}, diagnostics.NewError(diagnostic)
	}
	return app.downloads.Queue(app.context(), download.Request{
		ID:        request.ID,
		URL:       stream.URL,
		OutputDir: request.OutputDir,
		Headers:   headers,
	})
}

func (app *App) context() context.Context {
	if app.ctx != nil {
		return app.ctx
	}
	return context.Background()
}
