package download

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

type job struct {
	snapshot     JobSnapshot
	cancel       context.CancelFunc
	done         chan struct{}
	finalPath    string
	workPath     string
	lastProgress time.Time
}

// Service is the source of truth for local download jobs.
type Service struct {
	mu             sync.RWMutex
	jobs           map[string]*job
	order          []string
	reservations   map[string]string
	listeners      map[uint64]Listener
	nextListener   uint64
	config         Config
	limiter        *transport.ConnectionLimiter
	technicalStore *diagnostics.TechnicalStore
}

// NewService creates a download service with safe desktop defaults.
func NewService(config Config) *Service {
	if config.DefaultOutputDir == nil {
		config.DefaultOutputDir = defaultOutputDir
	}
	if config.ConnectionLimit <= 0 {
		config.ConnectionLimit = defaultConnectionLimit
	}
	if config.DefaultWorkers <= 0 {
		config.DefaultWorkers = defaultWorkerCount
	}
	return &Service{
		jobs:           make(map[string]*job),
		reservations:   make(map[string]string),
		listeners:      make(map[uint64]Listener),
		config:         config,
		limiter:        transport.NewConnectionLimiter(config.ConnectionLimit),
		technicalStore: config.TechnicalStore,
	}
}

// Subscribe registers a local event listener and returns an unsubscribe function.
func (service *Service) Subscribe(listener Listener) func() {
	if listener == nil {
		return func() {}
	}
	service.mu.Lock()
	id := service.nextListener
	service.nextListener++
	service.listeners[id] = listener
	service.mu.Unlock()
	return func() {
		service.mu.Lock()
		delete(service.listeners, id)
		service.mu.Unlock()
	}
}

// Queue validates, reserves, and starts a local download job.
func (service *Service) Queue(parent context.Context, request Request) (JobSnapshot, error) {
	if parent == nil {
		parent = context.Background()
	}
	if strings.TrimSpace(request.ID) == "" {
		return JobSnapshot{}, fmt.Errorf("download id is required")
	}
	if err := validateSourceURL(request.URL); err != nil {
		return JobSnapshot{}, err
	}

	kind := media.ClassifyURL(request.URL)
	directory, err := service.outputDirectory(request.OutputDir)
	if err != nil {
		return JobSnapshot{}, err
	}
	filename := outputFilename(request, kind)
	finalPath, workPath, err := reserveOutputPath(directory, filename, request.ID)
	if err != nil {
		return JobSnapshot{}, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return JobSnapshot{}, fmt.Errorf("create output folder: %w", err)
	}

	service.mu.Lock()
	if _, exists := service.jobs[request.ID]; exists {
		service.mu.Unlock()
		return JobSnapshot{}, &ConflictError{Resource: "download id", Value: request.ID}
	}
	if owner, reserved := service.reservations[finalPath]; reserved {
		service.mu.Unlock()
		return JobSnapshot{}, &ConflictError{Resource: "output path reserved by " + owner, Value: finalPath}
	}
	if _, err := os.Lstat(finalPath); err == nil && !request.Overwrite {
		service.mu.Unlock()
		return JobSnapshot{}, &ConflictError{Resource: "output path", Value: finalPath}
	} else if err != nil && !os.IsNotExist(err) {
		service.mu.Unlock()
		return JobSnapshot{}, fmt.Errorf("inspect output path: %w", err)
	}
	if _, err := os.Lstat(workPath); err == nil {
		service.mu.Unlock()
		return JobSnapshot{}, &ConflictError{Resource: "temporary output path", Value: workPath}
	} else if err != nil && !os.IsNotExist(err) {
		service.mu.Unlock()
		return JobSnapshot{}, fmt.Errorf("inspect temporary output path: %w", err)
	}

	ctx, cancel := context.WithCancel(parent)
	jobState := &job{
		snapshot: JobSnapshot{
			ID:         request.ID,
			Name:       filename,
			OutputPath: finalPath,
			Status:     StatusQueued,
			Version:    1,
		},
		cancel:    cancel,
		done:      make(chan struct{}),
		finalPath: finalPath,
		workPath:  workPath,
	}
	service.jobs[request.ID] = jobState
	service.order = append(service.order, request.ID)
	service.reservations[finalPath] = request.ID
	snapshot := jobState.snapshot
	service.mu.Unlock()

	service.emit(snapshot)
	go service.run(ctx, request, kind)
	return snapshot, nil
}

// Wait blocks until the job reaches a terminal state or ctx is cancelled.
func (service *Service) Wait(ctx context.Context, id string) (JobSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	service.mu.RLock()
	jobState, ok := service.jobs[id]
	if !ok {
		service.mu.RUnlock()
		return JobSnapshot{}, fmt.Errorf("download was not found: %s", id)
	}
	snapshot := jobState.snapshot
	done := jobState.done
	service.mu.RUnlock()
	if isTerminal(snapshot.Status) {
		return snapshot, nil
	}
	select {
	case <-done:
		service.mu.RLock()
		snapshot = service.jobs[id].snapshot
		service.mu.RUnlock()
		return snapshot, nil
	case <-ctx.Done():
		return JobSnapshot{}, ctx.Err()
	}
}

// Cancel requests cancellation for an active job and returns its latest snapshot.
func (service *Service) Cancel(id string) (JobSnapshot, error) {
	service.mu.RLock()
	jobState, ok := service.jobs[id]
	if !ok {
		service.mu.RUnlock()
		return JobSnapshot{}, fmt.Errorf("download was not found: %s", id)
	}
	cancel := jobState.cancel
	snapshot := jobState.snapshot
	service.mu.RUnlock()
	if cancel != nil && !isTerminal(snapshot.Status) {
		cancel()
	}
	return snapshot, nil
}

// List returns ordered snapshots for client hydration.
func (service *Service) List() []JobSnapshot {
	service.mu.RLock()
	defer service.mu.RUnlock()
	snapshots := make([]JobSnapshot, 0, len(service.order))
	for _, id := range service.order {
		if jobState, ok := service.jobs[id]; ok {
			snapshots = append(snapshots, jobState.snapshot)
		}
	}
	return snapshots
}

// CancelAll stops active jobs during application shutdown.
func (service *Service) CancelAll() {
	service.mu.RLock()
	cancels := make([]context.CancelFunc, 0, len(service.jobs))
	for _, jobState := range service.jobs {
		if jobState.cancel != nil && !isTerminal(jobState.snapshot.Status) {
			cancels = append(cancels, jobState.cancel)
		}
	}
	service.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (service *Service) run(ctx context.Context, request Request, kind media.SourceKind) {
	service.setStatus(request.ID, StatusDownloading, "")
	service.mu.RLock()
	jobState, ok := service.jobs[request.ID]
	service.mu.RUnlock()
	if !ok {
		return
	}

	err := service.execute(ctx, request, kind, jobState.workPath)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err == nil {
		err = publishOutput(jobState.workPath, jobState.finalPath, request.ID, request.Overwrite)
	}
	if err != nil {
		_ = os.Remove(jobState.workPath)
	}

	if err == nil {
		service.finish(request.ID, StatusCompleted, nil)
		return
	}

	cancelled := errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
	if cancelled && !errors.Is(err, context.Canceled) {
		err = context.Canceled
	}
	diagnostic := service.diagnosticForFailure(request, kind, err)
	if cancelled {
		service.finish(request.ID, StatusCancelled, diagnostic)
		return
	}
	service.finish(request.ID, StatusFailed, diagnostic)
}

func (service *Service) execute(ctx context.Context, request Request, kind media.SourceKind, workPath string) error {
	client := transport.NewHTTPClient(transport.HTTPClientConfig{
		UserAgent:  request.UserAgent,
		ProxyURL:   request.ProxyURL,
		Headers:    request.Headers,
		Cookie:     request.Cookie,
		CookieFile: request.CookieFile,
		Limiter:    service.limiter,
	})
	workers := request.Workers
	if workers <= 0 {
		workers = service.config.DefaultWorkers
	}
	progress := func(update downloader.Progress) {
		update.ActiveConnections = service.limiter.Active()
		service.setProgress(request.ID, update)
	}

	switch kind {
	case media.SourceHLS:
		return service.downloadHLS(ctx, client, request, workPath, workers, progress)
	case media.SourceDASH:
		return service.downloadDASH(ctx, client, request, workPath, workers, progress)
	default:
		return downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{
			Workers:    workers,
			OnProgress: progress,
		}).Download(ctx, request.URL, workPath)
	}
}

func (service *Service) downloadHLS(ctx context.Context, client *transport.HTTPClient, request Request, workPath string, workers int, progress downloader.ProgressCallback) error {
	body, err := client.Get(ctx, request.URL)
	if err != nil {
		return fmt.Errorf("fetch HLS playlist: %w", err)
	}
	playlist, err := parser.ParseHLS(body, request.URL)
	if err != nil {
		return fmt.Errorf("parse HLS playlist: %w", err)
	}
	for playlist.IsMaster {
		variant, selectErr := selectHLSVariant(playlist, request.Format)
		if selectErr != nil {
			return selectErr
		}
		body, err = client.Get(ctx, variant.URI)
		if err != nil {
			return fmt.Errorf("fetch HLS variant: %w", err)
		}
		playlist, err = parser.ParseHLS(body, variant.URI)
		if err != nil {
			return fmt.Errorf("parse HLS variant: %w", err)
		}
	}
	return downloader.NewHLSDownloader(client, downloader.HLSDownloaderConfig{
		Workers:    workers,
		OnProgress: progress,
	}).Download(ctx, playlist, workPath)
}

func (service *Service) downloadDASH(ctx context.Context, client *transport.HTTPClient, request Request, workPath string, workers int, progress downloader.ProgressCallback) error {
	body, err := client.Get(ctx, request.URL)
	if err != nil {
		return fmt.Errorf("fetch DASH manifest: %w", err)
	}
	manifest, err := parser.ParseDASH(body, request.URL)
	if err != nil {
		return fmt.Errorf("parse DASH manifest: %w", err)
	}
	representation, err := selectDASHRepresentation(manifest, request.Format)
	if err != nil {
		return err
	}
	return downloader.NewDASHDownloader(client, downloader.DASHDownloaderConfig{
		Workers:    workers,
		OnProgress: progress,
	}).Download(ctx, representation, workPath)
}

func (service *Service) outputDirectory(directory string) (string, error) {
	if strings.TrimSpace(directory) != "" {
		return directory, nil
	}
	return service.config.DefaultOutputDir()
}

func outputFilename(request Request, kind media.SourceKind) string {
	filename := request.Filename
	if strings.TrimSpace(filename) == "" {
		return media.TimestampedOutputFilename(request.URL, kind, time.Now())
	}
	if extension := media.DefaultExtension(kind); extension != "" {
		return media.WithExtension(filename, extension)
	}
	return media.SanitizeFilename(filename)
}

func validateSourceURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("enter a valid http or https URL")
	}
	return nil
}

func defaultOutputDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user folder: %w", err)
	}
	return filepath.Join(home, "Downloads"), nil
}

func selectHLSVariant(playlist *parser.HLSPlaylist, selection string) (*parser.HLSVariant, error) {
	formats := make([]media.Format, len(playlist.Variants))
	for index, variant := range playlist.Variants {
		formats[index] = media.Format{
			ID:        strconv.Itoa(index),
			Bandwidth: variant.Bandwidth,
			Width:     variant.Resolution.Width,
			Height:    variant.Resolution.Height,
		}
	}
	index, ok := media.SelectFormatIndex(formats, selection)
	if !ok {
		return nil, fmt.Errorf("HLS format was not found: %s", selection)
	}
	return &playlist.Variants[index], nil
}

func selectDASHRepresentation(manifest *parser.DASHManifest, selection string) (*parser.DASHRepresentation, error) {
	representations := make([]*parser.DASHRepresentation, 0)
	formats := make([]media.Format, 0)
	for periodIndex := range manifest.Periods {
		period := &manifest.Periods[periodIndex]
		for setIndex := range period.AdaptationSets {
			adaptationSet := &period.AdaptationSets[setIndex]
			if adaptationSet.MimeType != "" && !strings.HasPrefix(adaptationSet.MimeType, "video/") {
				continue
			}
			for representationIndex := range adaptationSet.Representations {
				representation := &adaptationSet.Representations[representationIndex]
				representations = append(representations, representation)
				formats = append(formats, media.Format{
					ID:        representation.ID,
					Bandwidth: representation.Bandwidth,
					Width:     representation.Width,
					Height:    representation.Height,
				})
			}
		}
	}
	index, ok := media.SelectFormatIndex(formats, selection)
	if !ok {
		return nil, fmt.Errorf("DASH format was not found: %s", selection)
	}
	return representations[index], nil
}

func (service *Service) setStatus(id string, status Status, message string) {
	service.update(id, func(snapshot *JobSnapshot) {
		snapshot.Status = status
		snapshot.Message = message
	})
}

func (service *Service) setProgress(id string, progress downloader.Progress) {
	service.mu.Lock()
	jobState, ok := service.jobs[id]
	if !ok || isTerminal(jobState.snapshot.Status) {
		service.mu.Unlock()
		return
	}
	now := time.Now()
	jobState.snapshot.Progress = progress
	if !jobState.lastProgress.IsZero() && now.Sub(jobState.lastProgress) < 250*time.Millisecond {
		service.mu.Unlock()
		return
	}
	jobState.lastProgress = now
	jobState.snapshot.Version++
	snapshot := jobState.snapshot
	service.mu.Unlock()
	service.emit(snapshot)
}

func (service *Service) finish(id string, status Status, diagnostic *diagnostics.Diagnostic) {
	service.mu.Lock()
	jobState, ok := service.jobs[id]
	if !ok {
		service.mu.Unlock()
		return
	}
	jobState.snapshot.Status = status
	jobState.snapshot.Message = ""
	if status != StatusCompleted {
		jobState.snapshot.OutputPath = ""
	}
	jobState.snapshot.Diagnostic = cloneDiagnostic(diagnostic)
	if diagnostic != nil {
		jobState.snapshot.Message = diagnostic.UserMessage
	}
	jobState.snapshot.Version++
	jobState.cancel = nil
	delete(service.reservations, jobState.finalPath)
	snapshot := jobState.snapshot
	close(jobState.done)
	service.mu.Unlock()
	service.emit(snapshot)
}

func (service *Service) update(id string, update func(*JobSnapshot)) {
	service.mu.Lock()
	jobState, ok := service.jobs[id]
	if !ok || isTerminal(jobState.snapshot.Status) {
		service.mu.Unlock()
		return
	}
	update(&jobState.snapshot)
	jobState.snapshot.Version++
	snapshot := jobState.snapshot
	service.mu.Unlock()
	service.emit(snapshot)
}

func (service *Service) emit(snapshot JobSnapshot) {
	service.mu.RLock()
	listeners := make([]Listener, 0, len(service.listeners))
	for _, listener := range service.listeners {
		listeners = append(listeners, listener)
	}
	service.mu.RUnlock()
	for _, listener := range listeners {
		listener(snapshot)
	}
}

func isTerminal(status Status) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}
