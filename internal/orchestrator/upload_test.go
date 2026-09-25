package orchestrator_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

type env struct {
	ports        orchestrator.Ports
	validator    *stubs.ValidatorStub
	persistence  *stubs.PersistenceStub
	queue        *stubs.MemoryQueue
	persistCache *clients.Persistence
}

// newEnv wires the real HTTP clients to the in-memory stubs and the memory
// queue, so the orchestrator core runs against fake downstream services.
func newEnv(t *testing.T) *env {
	t.Helper()

	vs := stubs.NewValidatorStub()
	ps := stubs.NewPersistenceStub()
	q := stubs.NewMemoryQueue()
	pc := clients.NewPersistence(ps.URL)

	t.Cleanup(func() {
		vs.Close()
		ps.Close()
	})

	return &env{
		ports: orchestrator.Ports{
			Validator:   clients.NewValidator(vs.URL),
			Persistence: pc,
			Queue:       q,
		},
		validator:    vs,
		persistence:  ps,
		queue:        q,
		persistCache: pc,
	}
}

func TestSubmitPDF(t *testing.T) {
	e := newEnv(t)
	content := []byte("%PDF-1.4\nfake")

	sum, err := orchestrator.Submit(context.Background(), e.ports, content, "informe.pdf", "informe")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if sum.Status != dto.StatusPending {
		t.Errorf("Status = %q, want pending", sum.Status)
	}
	if sum.OriginalFormat != dto.FormatPDF {
		t.Errorf("OriginalFormat = %q, want pdf", sum.OriginalFormat)
	}
	if sum.Title != "informe" {
		t.Errorf("Title = %q, want informe", sum.Title)
	}

	jobs := e.queue.PublishedExtractionJobs()
	if len(jobs) != 1 {
		t.Fatalf("published jobs = %d, want 1", len(jobs))
	}
	job := jobs[0]
	if job.PdfID != sum.ID {
		t.Errorf("job.PdfID = %q, want %q", job.PdfID, sum.ID)
	}
	if job.Filename != "informe.pdf" {
		t.Errorf("job.Filename = %q, want informe.pdf", job.Filename)
	}
	if !bytes.Equal(job.ContentBase64, content) {
		t.Errorf("job.ContentBase64 = %q, want %q", job.ContentBase64, content)
	}
}

func TestSubmitMarkdown(t *testing.T) {
	e := newEnv(t)
	content := []byte("# título\n\ntexto")

	sum, err := orchestrator.Submit(context.Background(), e.ports, content, "nota.md", "nota")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if sum.Status != dto.StatusDone {
		t.Errorf("Status = %q, want done", sum.Status)
	}
	if sum.OriginalFormat != dto.FormatMarkdown {
		t.Errorf("OriginalFormat = %q, want markdown", sum.OriginalFormat)
	}

	if jobs := e.queue.PublishedExtractionJobs(); len(jobs) != 0 {
		t.Errorf("published jobs = %d, want 0 (markdown must not hit the queue)", len(jobs))
	}

	rec, err := e.persistCache.Get(context.Background(), sum.ID)
	if err != nil {
		t.Fatalf("Get persisted: %v", err)
	}
	if rec.Content == nil || *rec.Content != string(content) {
		t.Errorf("persisted content = %v, want %q", rec.Content, content)
	}
}

func TestSubmitDuplicate(t *testing.T) {
	e := newEnv(t)
	content := []byte("%PDF-1.4\nsame")

	if _, err := orchestrator.Submit(context.Background(), e.ports, content, "a.pdf", "a"); err != nil {
		t.Fatalf("first Submit: %v", err)
	}

	_, err := orchestrator.Submit(context.Background(), e.ports, content, "b.pdf", "b")
	if !errors.Is(err, orchestrator.ErrDuplicate) {
		t.Fatalf("err = %v, want ErrDuplicate", err)
	}

	var dup orchestrator.DuplicateError
	if !errors.As(err, &dup) {
		t.Fatalf("err = %v, want DuplicateError", err)
	}
	if dup.ID == "" {
		t.Error("DuplicateError.ID is empty (existing_id missing)")
	}
}

func TestSubmitInvalid(t *testing.T) {
	e := newEnv(t)
	e.validator.FailStatus = 400

	_, err := orchestrator.Submit(context.Background(), e.ports, []byte("no es pdf"), "x.pdf", "x")
	if !errors.Is(err, orchestrator.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestSubmitDownstream(t *testing.T) {
	e := newEnv(t)
	e.validator.FailStatus = 500

	_, err := orchestrator.Submit(context.Background(), e.ports, []byte("%PDF-1.4"), "x.pdf", "x")
	if !errors.Is(err, orchestrator.ErrDownstream) {
		t.Fatalf("err = %v, want ErrDownstream", err)
	}
}
