package main

import (
	"context"
	"fmt"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/util"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Download downloads direct media, HLS playlists, or DASH manifests to outputDir.
func (a *App) Download(url string, outputDir string) error {
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
	})

	if outputDir == "" {
		outputDir = "download"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	outputPath := filepath.Join(outputDir, util.FilenameFromURL(url))

	switch strings.ToLower(filepath.Ext(parsedURL.Path)) {
	case ".m3u8":
		return downloadHLS(a.ctx, client, url, outputPath)
	case ".mpd":
		return downloadDASH(a.ctx, client, url, outputPath)
	default:
		eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{Workers: 8})
		return eng.Download(a.ctx, url, outputPath)
	}
}

func downloadHLS(ctx context.Context, client *util.HTTPClient, url, outputPath string) error {
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
		return downloadHLS(ctx, client, variant.URI, outputPath)
	}
	if strings.EqualFold(filepath.Ext(outputPath), ".m3u8") {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".ts"
	}
	return downloader.NewHLSDownloader(client, downloader.HLSDownloaderConfig{Workers: 8}).Download(ctx, playlist, outputPath)
}

func downloadDASH(ctx context.Context, client *util.HTTPClient, url, outputPath string) error {
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
	return downloader.NewDASHDownloader(client, downloader.DASHDownloaderConfig{Workers: 8}).Download(ctx, best, outputPath)
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
