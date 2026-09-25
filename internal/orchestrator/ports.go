// Package orchestrator contains the pure business logic of pdf-main. It
// depends only on the ports (interfaces) defined here, never on HTTP or queue
// adapters (ADR-0004).
package orchestrator

import (
	"context"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// Validator classifies uploaded content (PDF or Markdown) and computes its
// checksum. Implemented by clients.Validator.
type Validator interface {
	Validate(ctx context.Context, content []byte, filename string) (dto.ValidateResponse, error)
}

// Persistence is the gateway to pdf-persistance. Implemented by
// clients.Persistence.
type Persistence interface {
	Create(ctx context.Context, req dto.PersistCreateRequest) (dto.PersistRecord, error)
	FindByChecksum(ctx context.Context, checksum string) (*dto.PersistRecord, error)
	Get(ctx context.Context, id string) (dto.PersistRecord, error)
	List(ctx context.Context) ([]dto.PersistRecord, error)
	Update(ctx context.Context, id string, req dto.PersistUpdateRequest) (dto.PersistRecord, error)
	Delete(ctx context.Context, id string) error
}

// Converter converts Markdown to PDF on download (ADR-0005, download-only).
// Implemented by clients.Converter.
type Converter interface {
	Convert(ctx context.Context, content string) (dto.ConvertResponse, error)
}

// Producer publishes extraction jobs to the queue. Implemented by
// queue.Producer (and by the in-memory fake in test/stubs).
type Producer interface {
	PublishExtraction(ctx context.Context, job dto.ExtractionJob) error
}

// Ports groups the dependencies the orchestrator core needs.
type Ports struct {
	Validator   Validator
	Persistence Persistence
	Converter   Converter
	Queue       Producer
}
