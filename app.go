package main

import (
	"context"
	"fmt"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opendownload/opendownload/internal/capture"
	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/util"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	capture   *capture.Manager
	limiter   *util.ConnectionLimiter
	downloads sync.Map
}

type downloadProgressEvent struct {
	ID string `json:"id"`
	downloader.Progress
}

type downloadStateEvent struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type downloadDestinationEvent struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{capture: capture.NewManager(), limiter: util.NewConnectionLimiter(12)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(context.Context) {
	a.capture.Stop()
}

// Download downloads direct media, HLS playlists, or DASH manifests to outputDir.
func (a *App) Download(jobID string, url string, outputDir string) error {
	return a.download(a.ctx, jobID, url, outputDir, nil)
}

// QueueDownload returns after scheduling work. Progress and state are emitted
// through the Wails event bridge so the desktop can run several jobs at once.
func (a *App) QueueDownload(jobID string, url string, outputDir string) error {
	return a.queue(jobID, func(ctx context.Context) error { return a.download(ctx, jobID, url, outputDir, nil) })
}

func (a *App) CancelDownload(jobID string) {
	if value, ok := a.downloads.Load(jobID); ok {
		value.(context.CancelFunc)()
	}
}

func (a *App) StartFirefoxCapture() (capture.Pairing, error) {
	return a.capture.Start()
}

func (a *App) StopFirefoxCapture() {
	a.capture.Stop()
}

func (a *App) ListCapturedStreams() []capture.Stream {
	return a.capture.List()
}

func (a *App) DownloadCapturedStream(jobID string, capturedStreamID string, outputDir string) error {
	stream, headers, ok := a.capture.Get(capturedStreamID)
	if !ok {
		return fmt.Errorf("captured stream is no longer available")
	}
	if err := a.download(a.ctx, jobID, stream.URL, outputDir, headers); err != nil {
		if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
			return fmt.Errorf("stream access was rejected; capture a fresh request and try again: %w", err)
		}
		return err
	}
	return nil
}

func (a *App) QueueCapturedStream(jobID string, capturedStreamID string, outputDir string) error {
	stream, headers, ok := a.capture.Get(capturedStreamID)
	if !ok {
		return fmt.Errorf("captured stream is no longer available")
	}
	return a.queue(jobID, func(ctx context.Context) error { return a.download(ctx, jobID, stream.URL, outputDir, headers) })
}

func (a *App) queue(jobID string, work func(context.Context) error) error {
	if jobID == "" {
		return fmt.Errorf("download id is required")
	}
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	if _, loaded := a.downloads.LoadOrStore(jobID, cancel); loaded {
		cancel()
		return fmt.Errorf("a download with this id is already queued")
	}
	a.emitState(jobID, "queued", "")
	go func() {
		a.emitState(jobID, "downloading", "")
		err := work(ctx)
		switch {
		case err == nil:
			a.emitState(jobID, "completed", "")
		case ctx.Err() != nil:
			a.emitState(jobID, "cancelled", "")
		default:
			a.emitState(jobID, "failed", err.Error())
		}
		a.downloads.Delete(jobID)
		cancel()
	}()
	return nil
}

func (a *App) download(ctx context.Context, jobID string, url string, outputDir string, headers map[string]string) error {
	url = strings.TrimSpace(url)
	parsedURL, err := urlpkg.Parse(url)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("enter a valid http or https URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are supported")
	}

	client := util.NewHTTPClient(util.HTTPClientConfig{
		Verbose: false,
		Headers: headers,
		Limiter: a.limiter,
	})

	if outputDir == "" {
		outputDir, err = defaultDownloadDir()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	outputPath := filepath.Join(outputDir, util.FilenameFromURL(url))
	a.emitDestination(jobID, outputPath)
	progress := a.progressReporter(jobID)

	switch strings.ToLower(filepath.Ext(parsedURL.Path)) {
	case ".m3u8":
		return downloadHLS(ctx, client, url, outputPath, progress)
	case ".mpd":
		return downloadDASH(ctx, client, url, outputPath, progress)
	default:
		eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{Workers: 8, OnProgress: progress})
		return eng.Download(ctx, url, outputPath)
	}
}

func defaultDownloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find Windows user folder: %w", err)
	}
	return filepath.Join(home, "Downloads"), nil
}

func (a *App) progressReporter(jobID string) downloader.ProgressCallback {
	return func(progress downloader.Progress) {
		progress.ActiveConnections = a.limiter.Active()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "download:progress", downloadProgressEvent{ID: jobID, Progress: progress})
		}
	}
}

func (a *App) emitState(jobID, state, message string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "download:state", downloadStateEvent{ID: jobID, State: state, Message: message})
	}
}

func (a *App) emitDestination(jobID, outputPath string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "download:destination", downloadDestinationEvent{ID: jobID, Path: outputPath})
	}
}

func downloadHLS(ctx context.Context, client *util.HTTPClient, url, outputPath string, progress downloader.ProgressCallback) error {
	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("fetch HLS playlist: %w", err)
	}
	playlist, err := parser.ParseHLS(body, url)
	if err != nil {
		return err
	}
	if playlist.IsMaster {
		variant := bestHLSVariant(playlist)
		if variant == nil {
			return fmt.Errorf("no HLS variant found")
		}
		return downloadHLS(ctx, client, variant.URI, outputPath, progress)
	}
	if strings.EqualFold(filepath.Ext(outputPath), ".m3u8") {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".ts"
	}
	return downloader.NewHLSDownloader(client, downloader.HLSDownloaderConfig{Workers: 8, OnProgress: progress}).Download(ctx, playlist, outputPath)
}

func downloadDASH(ctx context.Context, client *util.HTTPClient, url, outputPath string, progress downloader.ProgressCallback) error {
	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("fetch DASH manifest: %w", err)
	}
	manifest, err := parser.ParseDASH(body, url)
	if err != nil {
		return err
	}
	var best *parser.DASHRepresentation
	for _, period := range manifest.Periods {
		for _, adaptationSet := range period.AdaptationSets {
			if adaptationSet.MimeType != "" && !strings.HasPrefix(adaptationSet.MimeType, "video/") {
				continue
			}
			for i := range adaptationSet.Representations {
				representation := &adaptationSet.Representations[i]
				if best == nil || representation.Bandwidth > best.Bandwidth {
					best = representation
				}
			}
		}
	}
	if best == nil {
		return fmt.Errorf("no DASH video representation found")
	}
	if strings.EqualFold(filepath.Ext(outputPath), ".mpd") {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".mp4"
	}
	return downloader.NewDASHDownloader(client, downloader.DASHDownloaderConfig{Workers: 8, OnProgress: progress}).Download(ctx, best, outputPath)
}

func bestHLSVariant(playlist *parser.HLSPlaylist) *parser.HLSVariant {
	if len(playlist.Variants) == 0 {
		return nil
	}
	best := &playlist.Variants[0]
	for i := range playlist.Variants[1:] {
		candidate := &playlist.Variants[i+1]
		if candidate.Bandwidth > best.Bandwidth {
			best = candidate
		}
	}
	return best
}
