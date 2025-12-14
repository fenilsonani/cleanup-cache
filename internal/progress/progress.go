package progress

import (
	"fmt"
	"sync"
	"time"
)

// Phase represents the current phase of operation
type Phase string

const (
	PhaseScanning Phase = "scanning"
	PhaseCleaning Phase = "cleaning"
	PhaseComplete Phase = "complete"
	PhaseError    Phase = "error"
)

// ScanProgress represents progress during scanning
type ScanProgress struct {
	Phase           Phase
	Category        string
	CurrentPath     string
	FilesFound      int
	TotalSize       int64
	CategoriesTotal int
	CategoriesDone  int
	StartTime       time.Time
	Error           error
}

// CleanProgress represents progress during cleanup
type CleanProgress struct {
	Phase        Phase
	CurrentFile  string
	DeletedFiles int
	TotalFiles   int
	DeletedSize  int64
	TotalSize    int64
	SkippedFiles int
	ErrorCount   int
	StartTime    time.Time
	UsingSudo    bool
	SudoPrompted bool
	Error        error
}

// ProgressReporter provides thread-safe progress reporting
type ProgressReporter struct {
	scanProgress  *ScanProgress
	cleanProgress *CleanProgress
	mu            sync.RWMutex
	listeners     []chan interface{}
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		listeners: make([]chan interface{}, 0),
	}
}

// Subscribe returns a channel that receives progress updates
func (pr *ProgressReporter) Subscribe() <-chan interface{} {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	ch := make(chan interface{}, 10)
	pr.listeners = append(pr.listeners, ch)
	return ch
}

// Unsubscribe closes and removes a listener channel
func (pr *ProgressReporter) Unsubscribe(ch <-chan interface{}) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	for i, listener := range pr.listeners {
		if listener == ch {
			close(listener)
			pr.listeners = append(pr.listeners[:i], pr.listeners[i+1:]...)
			return
		}
	}
}

// UpdateScanProgress updates scan progress and notifies listeners
func (pr *ProgressReporter) UpdateScanProgress(update *ScanProgress) {
	pr.mu.Lock()
	pr.scanProgress = update
	listeners := make([]chan interface{}, len(pr.listeners))
	copy(listeners, pr.listeners)
	pr.mu.Unlock()

	// Notify all listeners (non-blocking)
	for _, listener := range listeners {
		select {
		case listener <- update:
		default:
			// Skip if channel is full
		}
	}
}

// UpdateCleanProgress updates clean progress and notifies listeners
func (pr *ProgressReporter) UpdateCleanProgress(update *CleanProgress) {
	pr.mu.Lock()
	pr.cleanProgress = update
	listeners := make([]chan interface{}, len(pr.listeners))
	copy(listeners, pr.listeners)
	pr.mu.Unlock()

	// Notify all listeners (non-blocking)
	for _, listener := range listeners {
		select {
		case listener <- update:
		default:
			// Skip if channel is full
		}
	}
}

// GetScanProgress returns the current scan progress
func (pr *ProgressReporter) GetScanProgress() *ScanProgress {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.scanProgress
}

// GetCleanProgress returns the current clean progress
func (pr *ProgressReporter) GetCleanProgress() *CleanProgress {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.cleanProgress
}

// FormatScanProgress returns a human-readable scan progress string
func FormatScanProgress(p *ScanProgress) string {
	if p == nil {
		return "Initializing..."
	}

	elapsed := time.Since(p.StartTime)

	switch p.Phase {
	case PhaseScanning:
		return fmt.Sprintf("Scanning %s... Found %d files (%s) [%s]",
			p.Category,
			p.FilesFound,
			FormatBytes(p.TotalSize),
			FormatDuration(elapsed))
	case PhaseComplete:
		return fmt.Sprintf("Scan complete: %d files (%s) in %s",
			p.FilesFound,
			FormatBytes(p.TotalSize),
			FormatDuration(elapsed))
	case PhaseError:
		return fmt.Sprintf("Scan error: %v", p.Error)
	default:
		return "Scanning..."
	}
}

// FormatCleanProgress returns a human-readable clean progress string
func FormatCleanProgress(p *CleanProgress) string {
	if p == nil {
		return "Preparing..."
	}

	elapsed := time.Since(p.StartTime)

	switch p.Phase {
	case PhaseCleaning:
		percentage := 0
		if p.TotalFiles > 0 {
			percentage = (p.DeletedFiles * 100) / p.TotalFiles
		}

		eta := ""
		if p.DeletedFiles > 0 && p.TotalFiles > p.DeletedFiles {
			avgTime := elapsed / time.Duration(p.DeletedFiles)
			remaining := time.Duration(p.TotalFiles-p.DeletedFiles) * avgTime
			eta = fmt.Sprintf(" ETA: %s", FormatDuration(remaining))
		}

		sudo := ""
		if p.UsingSudo {
			sudo = " [SUDO]"
		}

		return fmt.Sprintf("Cleaning... %d/%d files (%d%%) - %s freed%s%s",
			p.DeletedFiles,
			p.TotalFiles,
			percentage,
			FormatBytes(p.DeletedSize),
			sudo,
			eta)
	case PhaseComplete:
		return fmt.Sprintf("Cleanup complete: %d files deleted (%s) in %s",
			p.DeletedFiles,
			FormatBytes(p.DeletedSize),
			FormatDuration(elapsed))
	case PhaseError:
		return fmt.Sprintf("Cleanup error: %v", p.Error)
	default:
		return "Preparing cleanup..."
	}
}

// FormatBytes formats bytes in human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

// FormatDuration formats duration in human-readable format
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ProgressCallback is a function type for progress callbacks
type ProgressCallback func(deleted int, size int64)
