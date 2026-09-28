package orchestrator_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

func TestDownloadMarkdown(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	md := "# título\n\ntexto"
	rec := seed(t, e, "nota", dto.StatusDone, &md)

	res, err := orchestrator.Download(ctx, e.ports, rec.ID, dto.FormatMarkdown)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if res.MimeType != "text/markdown" {
		t.Errorf("MimeType = %q, want text/markdown", res.MimeType)
	}
	if res.Filename != "nota.md" {
		t.Errorf("Filename = %q, want nota.md", res.Filename)
	}
	if !bytes.Equal(res.Content, []byte(md)) {
		t.Errorf("Content = %q, want %q", res.Content, md)
	}
}

func TestDownloadPDF(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	md := "# título"
	rec := seed(t, e, "nota", dto.StatusDone, &md)

	res, err := orchestrator.Download(ctx, e.ports, rec.ID, dto.FormatPDF)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if res.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want application/pdf", res.MimeType)
	}
	if res.Filename != "nota.pdf" {
		t.Errorf("Filename = %q, want nota.pdf", res.Filename)
	}
	if !bytes.Equal(res.Content, e.converter.PDF) {
		t.Errorf("Content = %q, want the converter output", res.Content)
	}
}

func TestDownloadPending(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	rec := seed(t, e, "pending", dto.StatusPending, nil)

	if _, err := orchestrator.Download(ctx, e.ports, rec.ID, dto.FormatMarkdown); !errors.Is(err, orchestrator.ErrNotReady) {
		t.Fatalf("err = %v, want ErrNotReady", err)
	}
}

func TestDownloadFailed(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	rec := seed(t, e, "failed", dto.StatusFailed, nil)

	if _, err := orchestrator.Download(ctx, e.ports, rec.ID, dto.FormatMarkdown); !errors.Is(err, orchestrator.ErrFailed) {
		t.Fatalf("err = %v, want ErrFailed", err)
	}
}

func TestDownloadNotFound(t *testing.T) {
	e := newEnv(t)

	if _, err := orchestrator.Download(context.Background(), e.ports, "no-existe", dto.FormatMarkdown); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDownloadInvalidFormat(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	md := "# x"
	rec := seed(t, e, "x", dto.StatusDone, &md)

	if _, err := orchestrator.Download(ctx, e.ports, rec.ID, dto.Format("exe")); !errors.Is(err, orchestrator.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func seed(t *testing.T, e *env, title string, status dto.Status, content *string) dto.PersistRecord {
	t.Helper()
	rec, err := e.persistCache.Create(context.Background(), dto.PersistCreateRequest{
		Title:          title,
		OriginalFormat: dto.FormatPDF,
		Checksum:       "checksum-" + title,
		Status:         status,
		Content:        content,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return rec
}
