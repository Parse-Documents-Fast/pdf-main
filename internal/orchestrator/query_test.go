package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

func TestList(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	md := "# b"
	if _, err := e.persistCache.Create(ctx, dto.PersistCreateRequest{
		Title: "a", OriginalFormat: dto.FormatPDF, Checksum: "c1", Status: dto.StatusPending,
	}); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if _, err := e.persistCache.Create(ctx, dto.PersistCreateRequest{
		Title: "b", OriginalFormat: dto.FormatMarkdown, Checksum: "c2", Status: dto.StatusDone, Content: &md,
	}); err != nil {
		t.Fatalf("seed b: %v", err)
	}

	sums, err := orchestrator.List(ctx, e.ports)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("len = %d, want 2", len(sums))
	}
	if sums[0].Title != "a" || sums[1].Title != "b" {
		t.Errorf("sums = %+v, want ordered by id", sums)
	}
}

func TestGet(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	md := "# título\n\ntexto"
	rec, err := e.persistCache.Create(ctx, dto.PersistCreateRequest{
		Title: "nota", OriginalFormat: dto.FormatMarkdown, Checksum: "c1", Status: dto.StatusDone, Content: &md,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	doc, err := orchestrator.Get(ctx, e.ports, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc.ID != rec.ID {
		t.Errorf("ID = %q, want %q", doc.ID, rec.ID)
	}
	if doc.Content == nil || *doc.Content != md {
		t.Errorf("Content = %v, want %q", doc.Content, md)
	}
}

func TestGetNotFound(t *testing.T) {
	e := newEnv(t)

	if _, err := orchestrator.Get(context.Background(), e.ports, "no-existe"); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	rec, err := e.persistCache.Create(ctx, dto.PersistCreateRequest{
		Title: "x", OriginalFormat: dto.FormatPDF, Checksum: "c1", Status: dto.StatusPending,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := orchestrator.Delete(ctx, e.ports, rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := e.persistCache.Get(ctx, rec.ID); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	e := newEnv(t)

	if err := orchestrator.Delete(context.Background(), e.ports, "no-existe"); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
