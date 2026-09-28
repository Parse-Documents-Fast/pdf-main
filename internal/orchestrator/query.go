package orchestrator

import (
	"context"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// List returns the public summaries of every stored document, delegating to
// the persistence gateway.
func List(ctx context.Context, p Ports) ([]dto.PdfSummary, error) {
	records, err := p.Persistence.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]dto.PdfSummary, 0, len(records))
	for _, r := range records {
		summaries = append(summaries, r.Summary())
	}
	return summaries, nil
}

// Get returns a single document by ID: the summary plus the Markdown content
// and the error detail (both null while the document is pending).
func Get(ctx context.Context, p Ports, id string) (dto.PdfDocument, error) {
	rec, err := p.Persistence.Get(ctx, id)
	if err != nil {
		return dto.PdfDocument{}, err
	}
	return rec.Document(), nil
}
