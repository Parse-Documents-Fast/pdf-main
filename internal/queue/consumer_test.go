package queue_test

import (
	"context"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
	"github.com/Parse-Documents-Fast/pdf-main/internal/queue"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

// newPersistence returns a real persistence client wired to the in-memory
// stub, so HandleResult can be exercised against a fake persistence.
func newPersistence(t *testing.T) *clients.Persistence {
	t.Helper()
	ps := stubs.NewPersistenceStub()
	t.Cleanup(ps.Close)
	return clients.NewPersistence(ps.URL)
}

func TestHandleResultDone(t *testing.T) {
	p := newPersistence(t)
	ctx := context.Background()

	rec, err := p.Create(ctx, dto.PersistCreateRequest{
		Title: "x", OriginalFormat: dto.FormatPDF, Checksum: "c", Status: dto.StatusPending,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	md := "# título\n\ntexto"
	if err := queue.HandleResult(ctx, dto.ExtractionResult{
		PdfID: rec.ID, Status: dto.StatusDone, Content: &md,
	}, p); err != nil {
		t.Fatalf("HandleResult: %v", err)
	}

	got, err := p.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != dto.StatusDone {
		t.Errorf("Status = %q, want done", got.Status)
	}
	if got.Content == nil || *got.Content != md {
		t.Errorf("Content = %v, want %q", got.Content, md)
	}
}

func TestHandleResultFailed(t *testing.T) {
	p := newPersistence(t)
	ctx := context.Background()

	rec, err := p.Create(ctx, dto.PersistCreateRequest{
		Title: "x", OriginalFormat: dto.FormatPDF, Checksum: "c", Status: dto.StatusPending,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	detail := "No se pudo extraer texto del PDF"
	if err := queue.HandleResult(ctx, dto.ExtractionResult{
		PdfID: rec.ID, Status: dto.StatusFailed, Error: &problem.ProblemDetails{Detail: detail},
	}, p); err != nil {
		t.Fatalf("HandleResult: %v", err)
	}

	got, err := p.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != dto.StatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.Error == nil || *got.Error != detail {
		t.Errorf("Error = %v, want %q", got.Error, detail)
	}
}

func TestHandleResultUnexpectedStatus(t *testing.T) {
	p := newPersistence(t)
	ctx := context.Background()

	rec, err := p.Create(ctx, dto.PersistCreateRequest{
		Title: "x", OriginalFormat: dto.FormatPDF, Checksum: "c", Status: dto.StatusPending,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := queue.HandleResult(ctx, dto.ExtractionResult{
		PdfID: rec.ID, Status: dto.Status("weird"),
	}, p); err == nil {
		t.Fatal("expected error for unexpected status, got nil")
	}
}
