package main

import (
	"context"
	"fmt"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/opendownload/opendownload/internal/capture"
	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/util"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	capture *capture.Manager
}

type downloadProgressEvent struct {
	ID string `json:"id"`
	downloader.Progress
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{capture: capture.NewManager()}
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
	return a.download(jobID, url, outputDir, nil)
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
	if err := a.download(jobID, stream.URL, outputDir, headers); err != nil {
		if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
			return fmt.Errorf("stream access was rejected; capture a fresh request and try again: %w", err)
		}
		return err
	}
	return nil
}

func (a *App) download(jobID string, url string, outputDir string, headers map[string]string) error {
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
	})

	if outputDir == "" {
		outputDir = "download"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	outputPath := filepath.Join(outputDir, util.FilenameFromURL(url))
	progress := a.progressReporter(jobID)

	switch strings.ToLower(filepath.Ext(parsedURL.Path)) {
	case ".m3u8":
		return downloadHLS(a.ctx, client, url, outputPath, progress)
	case ".mpd":
		return downloadDASH(a.ctx, client, url, outputPath, progress)
	default:
		eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{Workers: 8, OnProgress: progress})
		return eng.Download(a.ctx, url, outputPath)
	}
}

func (a *App) progressReporter(jobID string) downloader.ProgressCallback {
	return func(progress downloader.Progress) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "download:progress", downloadProgressEvent{ID: jobID, Progress: progress})
		}
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
