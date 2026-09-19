package download

import (
	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/downloader"
)

// Status is the authoritative lifecycle state for a download job.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusCompleted   Status = terminalStatusCompleted
	StatusFailed      Status = terminalStatusFailed
	StatusCancelled   Status = terminalStatusCancelled
)

// Request describes one download command. Headers and cookies are retained by
// the service and are never included in JobSnapshot events.
type Request struct {
	ID         string            `json:"id"`
	URL        string            `json:"url"`
	OutputDir  string            `json:"outputDir"`
	Filename   string            `json:"filename,omitempty"`
	Format     string            `json:"format,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Cookie     string            `json:"cookie,omitempty"`
	CookieFile string            `json:"cookieFile,omitempty"`
	UserAgent  string            `json:"userAgent,omitempty"`
	ProxyURL   string            `json:"proxyUrl,omitempty"`
	Workers    int               `json:"workers,omitempty"`
	Overwrite  bool              `json:"overwrite,omitempty"`
	// TechnicalContext resolves capture-owned detail only while diagnostics are
	// still enabled. It never crosses a public request boundary.
	TechnicalContext func() *diagnostics.TechnicalDiagnostic `json:"-"`
}

// JobSnapshot is the public, complete state of a download job.
type JobSnapshot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OutputPath string `json:"outputPath"`
	Status     Status `json:"status"`
	downloader.Progress
	Message    string                  `json:"message,omitempty"`
	Diagnostic *diagnostics.Diagnostic `json:"diagnostic,omitempty"`
	Version    uint64                  `json:"version"`
}

// Config configures a Service.
type Config struct {
	DefaultOutputDir func() (string, error)
	ConnectionLimit  int
	DefaultWorkers   int
	TechnicalStore   *diagnostics.TechnicalStore
}

// Listener receives a snapshot after it has been committed to service state.
type Listener func(JobSnapshot)
