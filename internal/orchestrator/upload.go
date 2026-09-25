package orchestrator

import (
	"context"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// Submit orchestrates an upload: validate, detect duplicates, then persist.
// Per ADR-0005 there are two paths:
//
//   - PDF: create the record as pending and publish an extraction job; the
//     Markdown arrives asynchronously via the queue.
//   - Markdown: persist synchronously as done, without touching the queue.
//
// title is the already-resolved document title (the HTTP adapter defaults it
// from the filename when the client omits it).
func Submit(ctx context.Context, p Ports, content []byte, filename, title string) (dto.PdfSummary, error) {
	v, err := p.Validator.Validate(ctx, content, filename)
	if err != nil {
		return dto.PdfSummary{}, err
	}

	dup, err := p.Persistence.FindByChecksum(ctx, v.Checksum)
	if err != nil {
		return dto.PdfSummary{}, err
	}
	if dup != nil {
		return dto.PdfSummary{}, DuplicateError{ID: dup.ID}
	}

	switch v.OriginalFormat {
	case dto.FormatPDF:
		rec, err := p.Persistence.Create(ctx, dto.PersistCreateRequest{
			Title:          title,
			OriginalFormat: v.OriginalFormat,
			Checksum:       v.Checksum,
			Status:         dto.StatusPending,
		})
		if err != nil {
			return dto.PdfSummary{}, err
		}

		if err := p.Queue.PublishExtraction(ctx, dto.ExtractionJob{
			PdfID:         rec.ID,
			Filename:      filename,
			ContentBase64: content,
		}); err != nil {
			return dto.PdfSummary{}, err
		}
		return rec.Summary(), nil

	case dto.FormatMarkdown:
		md := string(content)
		rec, err := p.Persistence.Create(ctx, dto.PersistCreateRequest{
			Title:          title,
			OriginalFormat: v.OriginalFormat,
			Checksum:       v.Checksum,
			Status:         dto.StatusDone,
			Content:        &md,
		})
		if err != nil {
			return dto.PdfSummary{}, err
		}
		return rec.Summary(), nil

	default:
		return dto.PdfSummary{}, ErrInvalid
	}
}
