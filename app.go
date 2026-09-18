package main

import (
	"context"
	"fmt"

	"github.com/opendownload/opendownload/internal/capture"
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
}

// NewApp creates the desktop application adapter.
func NewApp() *App {
	return &App{
		capture:   capture.NewManager(),
		downloads: download.NewService(download.Config{}),
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
}

func (app *App) shutdown(context.Context) {
	if app.stopDownloadEvents != nil {
		app.stopDownloadEvents()
	}
	if app.stopCaptureEvents != nil {
		app.stopCaptureEvents()
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
	return app.capture.Start()
}

// StopFirefoxCapture clears the active paired browser capture session.
func (app *App) StopFirefoxCapture() {
	app.capture.Stop()
}

// ListCapturedStreams returns public stream summaries without URLs or request headers.
func (app *App) ListCapturedStreams() []capture.StreamSummary {
	return app.capture.List()
}

// QueueCapturedStream schedules a download using request headers kept in memory by capture.
func (app *App) QueueCapturedStream(request CapturedDownloadRequest) (download.JobSnapshot, error) {
	stream, headers, ok := app.capture.Get(request.CapturedStreamID)
	if !ok {
		return download.JobSnapshot{}, fmt.Errorf("captured stream is no longer available")
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
