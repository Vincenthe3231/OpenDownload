package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/opendownload/opendownload/server/internal/downloader"
	"github.com/opendownload/opendownload/server/internal/parser"
	"github.com/opendownload/opendownload/server/internal/util"
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

// Download initiates a download
func (a *App) Download(url string, outputPath string) error {
	client := util.NewHTTPClient(util.HTTPClientConfig{
		Verbose: true,
	})

	// Naive detection - in production use the sniffer detection logic
	eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{
		Workers: 8,
		Verbose: true,
	})

	if outputPath == "" {
		outputPath = filepath.Join("./download", util.FilenameFromURL(url))
	}

	return eng.Download(a.ctx, url, outputPath)
}
