package stubs

import (
	"context"
	"sync"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// MemoryQueue is an in-memory fake of the Redis Streams used by the extraction
// pipeline. It records jobs published to queue:extraction and carries results
// published to queue:extraction-results (simulating pdf-extractor's side).
type MemoryQueue struct {
	mu      sync.Mutex
	jobs    []dto.ExtractionJob
	results chan dto.ExtractionResult
}

// NewMemoryQueue returns an empty in-memory queue.
func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		results: make(chan dto.ExtractionResult, 64),
	}
}

// PublishExtraction records a job in queue:extraction. It implements the
// Producer port consumed by the orchestrator.
func (q *MemoryQueue) PublishExtraction(_ context.Context, job dto.ExtractionJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

// PublishedExtractionJobs returns a copy of the jobs published so far, in
// order, for assertions.
func (q *MemoryQueue) PublishedExtractionJobs() []dto.ExtractionJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]dto.ExtractionJob, len(q.jobs))
	copy(out, q.jobs)
	return out
}

// PushResult simulates pdf-extractor publishing a result to
// queue:extraction-results.
func (q *MemoryQueue) PushResult(r dto.ExtractionResult) {
	q.results <- r
}

// Results returns the channel of results for the consumer to read.
func (q *MemoryQueue) Results() <-chan dto.ExtractionResult {
	return q.results
}
