package downloader

import (
	"fmt"
	"sync"
	"time"

	"github.com/opendownload/opendownload/internal/util"
)

const progressInterval = 250 * time.Millisecond

// Progress describes the current transfer state for a download.
type Progress struct {
	DownloadedBytes   int64   `json:"downloadedBytes"`
	TotalBytes        int64   `json:"totalBytes"`
	CompletedUnits    int64   `json:"completedUnits"`
	TotalUnits        int64   `json:"totalUnits"`
	BytesPerSecond    float64 `json:"bytesPerSecond"`
	ETASeconds        float64 `json:"etaSeconds"`
	HasETA            bool    `json:"hasEta"`
	ActiveConnections int64   `json:"activeConnections"`
}

// ProgressCallback receives periodic download progress updates.
type ProgressCallback func(Progress)

type progressReporter struct {
	callback  ProgressCallback
	start     time.Time
	mu        sync.Mutex
	last      time.Time
	latest    Progress
	hasLatest bool
}

func newProgressReporter(callback ProgressCallback) *progressReporter {
	return &progressReporter{callback: callback, start: time.Now()}
}

func (r *progressReporter) bytes(downloaded, total int64, force bool) {
	update := r.update(downloaded)
	update.TotalBytes = total
	if total > 0 && downloaded < total {
		update.ETASeconds = float64(total-downloaded) / update.BytesPerSecond
		update.HasETA = true
	}
	r.emit(update, force)
}

func (r *progressReporter) segments(downloaded, completed, total int64, force bool) {
	update := r.update(downloaded)
	update.CompletedUnits = completed
	update.TotalUnits = total
	if completed > 0 && completed < total {
		elapsed := time.Since(r.start).Seconds()
		update.ETASeconds = elapsed / float64(completed) * float64(total-completed)
		update.HasETA = true
	}
	r.emit(update, force)
}

func (r *progressReporter) update(downloaded int64) Progress {
	elapsed := time.Since(r.start).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	return Progress{
		DownloadedBytes: downloaded,
		BytesPerSecond:  float64(downloaded) / elapsed,
	}
}

func (r *progressReporter) emit(update Progress, force bool) {
	r.mu.Lock()
	if r.hasLatest {
		if update.DownloadedBytes < r.latest.DownloadedBytes {
			update.DownloadedBytes = r.latest.DownloadedBytes
		}
		if update.CompletedUnits < r.latest.CompletedUnits {
			update.CompletedUnits = r.latest.CompletedUnits
		}
	}
	if !force && !r.last.IsZero() && time.Since(r.last) < progressInterval {
		r.mu.Unlock()
		return
	}
	r.last = time.Now()
	r.latest = update
	r.hasLatest = true
	callback := r.callback
	if callback == nil {
		printProgress(update)
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	callback(update)
}

func (r *progressReporter) finish() {
	if r.callback == nil {
		fmt.Println()
	}
}

func printProgress(update Progress) {
	if update.TotalUnits > 0 {
		fmt.Printf("\r  Segments: %d/%d (%.1f%%) @ %s/s   ", update.CompletedUnits, update.TotalUnits, float64(update.CompletedUnits)/float64(update.TotalUnits)*100, util.FormatBytes(int64(update.BytesPerSecond)))
		return
	}
	if update.TotalBytes > 0 {
		fmt.Printf("\r  %s / %s (%.1f%%) @ %s/s   ", util.FormatBytes(update.DownloadedBytes), util.FormatBytes(update.TotalBytes), float64(update.DownloadedBytes)/float64(update.TotalBytes)*100, util.FormatBytes(int64(update.BytesPerSecond)))
		return
	}
	fmt.Printf("\r  %s @ %s/s   ", util.FormatBytes(update.DownloadedBytes), util.FormatBytes(int64(update.BytesPerSecond)))
}
