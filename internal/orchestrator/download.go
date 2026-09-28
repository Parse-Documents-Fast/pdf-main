package orchestrator

import (
	"context"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
)

// DownloadResult is the file to return to the client on download.
type DownloadResult struct {
	// Filename is the suggested attachment name, e.g. "informe.md".
	Filename string
	// MimeType is the Content-Type of the file.
	MimeType string
	// Content is the raw file content (Markdown text or PDF bytes).
	Content []byte
}

// Download builds the file for a document. Per ADR-0005 the canonical format
// is Markdown, so:
//
//   - format markdown: the stored Markdown is returned as-is, without touching
//     the converter.
//   - format pdf: the Markdown is converted to PDF via pdf-converter (the
//     converter is download-only; it is the last service that maps MD → PDF).
//
// A document that is not done cannot be downloaded.
func Download(ctx context.Context, p Ports, id string, format dto.Format) (DownloadResult, error) {
	rec, err := p.Persistence.Get(ctx, id)
	if err != nil {
		return DownloadResult{}, err
	}

	switch rec.Status {
	case dto.StatusDone:
		// proceed
	case dto.StatusPending:
		return DownloadResult{}, ErrNotReady
	case dto.StatusFailed:
		return DownloadResult{}, ErrFailed
	default:
		return DownloadResult{}, ErrNotReady
	}

	content := ""
	if rec.Content != nil {
		content = *rec.Content
	}

	switch format {
	case dto.FormatMarkdown:
		return DownloadResult{
			Filename: rec.Title + ".md",
			MimeType: "text/markdown",
			Content:  []byte(content),
		}, nil
	case dto.FormatPDF:
		resp, err := p.Converter.Convert(ctx, content)
		if err != nil {
			return DownloadResult{}, err
		}
		return DownloadResult{
			Filename: rec.Title + ".pdf",
			MimeType: resp.MimeType,
			Content:  resp.ContentBase64,
		}, nil
	default:
		return DownloadResult{}, ErrInvalid
	}
}
