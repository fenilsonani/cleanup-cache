package scanner

import (
	"context"
)

const (
	// DefaultBatchSize is the number of files to collect before emitting a batch
	DefaultBatchSize = 10000
	// ChannelBufferSize is the buffer size for streaming channels
	ChannelBufferSize = 10
)

// ScanBatch represents a batch of files found during streaming scan
type ScanBatch struct {
	Files     []FileInfo
	BatchSize int64  // Total size of files in this batch
	Category  string
	Error     error // If an error occurred during this batch
	Final     bool  // True if this is the last batch
}

// StreamingScanResult provides streaming access to scan results
type StreamingScanResult struct {
	Batches    <-chan *ScanBatch
	TotalFiles int64 // Updated as scan progresses
	TotalSize  int64 // Updated as scan progresses
	Categories []string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewStreamingScanResult creates a new streaming scan result
func NewStreamingScanResult(ctx context.Context) *StreamingScanResult {
	ctx, cancel := context.WithCancel(ctx)
	return &StreamingScanResult{
		Batches:    make(chan *ScanBatch, ChannelBufferSize),
		Categories: []string{},
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Cancel cancels the streaming scan
func (s *StreamingScanResult) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// BatchCollector helps collect files into batches for streaming
type BatchCollector struct {
	batchSize    int
	currentBatch []FileInfo
	currentSize  int64
	category     string
	channel      chan<- *ScanBatch
}

// NewBatchCollector creates a new batch collector
func NewBatchCollector(category string, batchSize int, channel chan<- *ScanBatch) *BatchCollector {
	return &BatchCollector{
		batchSize:    batchSize,
		currentBatch: make([]FileInfo, 0, batchSize),
		category:     category,
		channel:      channel,
	}
}

// Add adds a file to the current batch, emitting if batch is full
func (bc *BatchCollector) Add(file FileInfo) {
	bc.currentBatch = append(bc.currentBatch, file)
	bc.currentSize += file.Size

	if len(bc.currentBatch) >= bc.batchSize {
		bc.Flush()
	}
}

// Flush emits the current batch even if not full
func (bc *BatchCollector) Flush() {
	if len(bc.currentBatch) == 0 {
		return
	}

	batch := &ScanBatch{
		Files:     bc.currentBatch,
		BatchSize: bc.currentSize,
		Category:  bc.category,
		Final:     false,
	}

	bc.channel <- batch

	// Reset for next batch
	bc.currentBatch = make([]FileInfo, 0, bc.batchSize)
	bc.currentSize = 0
}

// Finalize sends the final batch and marks it as complete
func (bc *BatchCollector) Finalize() {
	if len(bc.currentBatch) > 0 {
		batch := &ScanBatch{
			Files:     bc.currentBatch,
			BatchSize: bc.currentSize,
			Category:  bc.category,
			Final:     true,
		}
		bc.channel <- batch
	} else {
		// Send empty final batch to signal completion
		batch := &ScanBatch{
			Files:     []FileInfo{},
			BatchSize: 0,
			Category:  bc.category,
			Final:     true,
		}
		bc.channel <- batch
	}

	bc.currentBatch = nil
}

// SendError sends an error batch
func (bc *BatchCollector) SendError(err error) {
	batch := &ScanBatch{
		Files:     []FileInfo{},
		BatchSize: 0,
		Category:  bc.category,
		Error:     err,
		Final:     true,
	}
	bc.channel <- batch
}

// CollectAllBatches collects all batches from a channel into a single ScanResult
// This provides backward compatibility for code that expects ScanResult
func CollectAllBatches(ctx context.Context, batchChan <-chan *ScanBatch) *ScanResult {
	result := &ScanResult{
		Files:  []FileInfo{},
		Errors: []error{},
	}

	for batch := range batchChan {
		// Check for cancellation
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			return result
		default:
		}

		if batch.Error != nil {
			result.Errors = append(result.Errors, batch.Error)
			continue
		}

		result.Files = append(result.Files, batch.Files...)
		result.TotalSize += batch.BatchSize
		result.TotalCount += len(batch.Files)
	}

	return result
}
