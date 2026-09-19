package main

import (
	"context"
	"time"

	"github.com/opendownload/opendownload/internal/capture"
	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/download"
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

// App is the thin Wails adapter around local application services.
type App struct {
	ctx                context.Context
	capture            *capture.Manager
	downloads          *download.Service
	stopDownloadEvents func()
	stopCaptureEvents  func()
	stopNativeCapture  func()
	diagnosticHistory  *diagnostics.History
}

// NewApp creates the desktop application adapter.
func NewApp() *App {
	return &App{
		capture:           capture.NewManager(),
		downloads:         download.NewService(download.Config{}),
		diagnosticHistory: diagnostics.NewHistory(),
	}
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
	app.stopNativeCapture, _ = app.capture.StartNativeServer(ctx)
}

func (app *App) shutdown(context.Context) {
	if app.stopDownloadEvents != nil {
		app.stopDownloadEvents()
	}
	if app.stopCaptureEvents != nil {
		app.stopCaptureEvents()
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

// StartFirefoxCapture starts a fresh paired browser capture session.
func (app *App) StartFirefoxCapture() (capture.Pairing, error) {
	pairing, err := app.capture.Start()
	if err != nil {
		app.setDiagnostic(diagnostics.New(diagnostics.CaptureRequestRejected, diagnostics.StageCapture, true, "Could not start browser capture.", diagnostics.NewID(), time.Now()).WithTechnicalDetail(err.Error()))
	}
	return pairing, err
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

// DeveloperDiagnosticsEnabled returns persisted developer diagnostics state.
func (app *App) DeveloperDiagnosticsEnabled() bool {
	return app.diagnosticHistory.Enabled()
}

// SetDeveloperDiagnostics controls technical detail persistence and display.
func (app *App) SetDeveloperDiagnostics(enabled bool) error {
	return app.diagnosticHistory.SetEnabled(enabled)
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
