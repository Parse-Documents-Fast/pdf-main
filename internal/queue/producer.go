// Package queue contains the Redis Streams adapters for the extraction
// pipeline (ADR-0004). The producer publishes extraction jobs; the consumer
// (added later) reads results. These adapters are deliberately thin: no
// business logic lives here.
package queue

import (
	"context"
	"encoding/base64"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/redis/go-redis/v9"
)

// Producer publishes extraction jobs to the inbound extraction stream
// (ADR-0005). It implements the orchestrator.Producer port.
type Producer struct {
	rdb    *redis.Client
	stream string
}

// NewProducer builds a producer connected to the Redis queue at addr, writing
// to the given stream name.
func NewProducer(addr, stream string) *Producer {
	return &Producer{
		rdb:    redis.NewClient(&redis.Options{Addr: addr}),
		stream: stream,
	}
}

// PublishExtraction writes a job to the configured extraction stream. The raw
// PDF travels base64-encoded in content_base64 (ADR-0002). This adapter is not
// unit tested; it is validated manually against pdf-infra.
func (p *Producer) PublishExtraction(ctx context.Context, job dto.ExtractionJob) error {
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]any{
			"pdf_id":         job.PdfID,
			"filename":       job.Filename,
			"content_base64": base64.StdEncoding.EncodeToString(job.ContentBase64),
		},
	}).Err()
}

// Close releases the underlying Redis connection pool.
func (p *Producer) Close() error {
	return p.rdb.Close()
}

var _ orchestrator.Producer = (*Producer)(nil)
