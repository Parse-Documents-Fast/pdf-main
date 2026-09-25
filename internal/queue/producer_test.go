package queue_test

import (
	"context"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

// The in-memory fake satisfies the same Producer port the orchestrator
// depends on, so unit tests run without a real Redis.
var _ orchestrator.Producer = (*stubs.MemoryQueue)(nil)

func TestMemoryQueueSatisfiesProducer(t *testing.T) {
	q := stubs.NewMemoryQueue()
	var p orchestrator.Producer = q

	job := dto.ExtractionJob{PdfID: "1", Filename: "informe.pdf", ContentBase64: []byte("%PDF-1.4")}
	if err := p.PublishExtraction(context.Background(), job); err != nil {
		t.Fatalf("PublishExtraction: %v", err)
	}

	jobs := q.PublishedExtractionJobs()
	if len(jobs) != 1 || jobs[0].PdfID != "1" || jobs[0].Filename != "informe.pdf" {
		t.Errorf("jobs = %+v, want the published extraction job", jobs)
	}
}
