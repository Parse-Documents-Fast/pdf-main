package clients_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

func TestValidatorValidateOK(t *testing.T) {
	s := stubs.NewValidatorStub()
	defer s.Close()

	c := clients.NewValidator(s.URL)
	got, err := c.Validate(context.Background(), []byte("%PDF-1.4\nfake"), "informe.pdf")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.OriginalFormat != dto.FormatPDF {
		t.Errorf("OriginalFormat = %q, want pdf", got.OriginalFormat)
	}
	if got.Checksum == "" {
		t.Error("Checksum empty")
	}
}

func TestValidatorValidateInvalid(t *testing.T) {
	s := stubs.NewValidatorStub()
	s.FailStatus = http.StatusBadRequest
	defer s.Close()

	c := clients.NewValidator(s.URL)
	_, err := c.Validate(context.Background(), []byte("no es pdf"), "x.pdf")
	if !errors.Is(err, orchestrator.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestValidatorValidateDownstream(t *testing.T) {
	s := stubs.NewValidatorStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()

	c := clients.NewValidator(s.URL)
	_, err := c.Validate(context.Background(), []byte("%PDF-1.4"), "x.pdf")
	if !errors.Is(err, orchestrator.ErrDownstream) {
		t.Fatalf("err = %v, want ErrDownstream", err)
	}
}

func TestValidatorCircuitBreakerOpens(t *testing.T) {
	s := stubs.NewValidatorStub()
	defer s.Close()
	c := clients.NewValidator(s.URL)

	// Trip the breaker with consecutive failures (no timer waits).
	s.FailStatus = http.StatusInternalServerError
	for i := 0; i < 7; i++ {
		if _, err := c.Validate(context.Background(), []byte("%PDF-1.4"), "x.pdf"); !errors.Is(err, orchestrator.ErrDownstream) {
			t.Fatalf("call %d: err = %v, want ErrDownstream", i, err)
		}
	}

	// The service is healthy again, but the breaker is open: the client must
	// fail fast without reaching downstream.
	s.FailStatus = 0
	if _, err := c.Validate(context.Background(), []byte("%PDF-1.4"), "x.pdf"); !errors.Is(err, orchestrator.ErrDownstream) {
		t.Fatalf("open breaker: err = %v, want ErrDownstream", err)
	}
}

func TestConverterConvertOK(t *testing.T) {
	s := stubs.NewConverterStub()
	defer s.Close()

	c := clients.NewConverter(s.URL)
	got, err := c.Convert(context.Background(), "# título\n\ntexto")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if got.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want application/pdf", got.MimeType)
	}
	if len(got.ContentBase64) == 0 {
		t.Error("ContentBase64 empty")
	}
}

func TestConverterConvertDownstream(t *testing.T) {
	s := stubs.NewConverterStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()

	c := clients.NewConverter(s.URL)
	_, err := c.Convert(context.Background(), "# t")
	if !errors.Is(err, orchestrator.ErrDownstream) {
		t.Fatalf("err = %v, want ErrDownstream", err)
	}
}

func TestPersistenceCRUD(t *testing.T) {
	s := stubs.NewPersistenceStub()
	defer s.Close()
	c := clients.NewPersistence(s.URL)
	ctx := context.Background()

	// Create.
	created, err := c.Create(ctx, dto.PersistCreateRequest{
		Title:          "informe",
		OriginalFormat: dto.FormatPDF,
		Checksum:       "abc123",
		Status:         dto.StatusPending,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.Status != dto.StatusPending {
		t.Errorf("created = %+v", created)
	}

	// Get.
	got, err := c.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.Title != "informe" {
		t.Errorf("got = %+v", got)
	}

	// FindByChecksum: found.
	found, err := c.FindByChecksum(ctx, "abc123")
	if err != nil {
		t.Fatalf("FindByChecksum: %v", err)
	}
	if found == nil || found.ID != created.ID {
		t.Errorf("found = %+v", found)
	}

	// FindByChecksum: not found → nil, no error.
	missing, err := c.FindByChecksum(ctx, "nope")
	if err != nil {
		t.Fatalf("FindByChecksum(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("missing = %+v, want nil", missing)
	}

	// Update.
	updated, err := c.Update(ctx, created.ID, dto.PersistUpdateRequest{
		Status:  dto.StatusDone,
		Content: strPtr("# título"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != dto.StatusDone || updated.Content == nil {
		t.Errorf("updated = %+v", updated)
	}

	// List.
	list, err := c.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list length = %d, want 1", len(list))
	}

	// Delete.
	if err := c.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get after delete → ErrNotFound.
	if _, err := c.Get(ctx, created.ID); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
}

func TestPersistenceNotFound(t *testing.T) {
	s := stubs.NewPersistenceStub()
	defer s.Close()
	c := clients.NewPersistence(s.URL)

	if _, err := c.Get(context.Background(), "no-existe"); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Errorf("Get: err = %v, want ErrNotFound", err)
	}
	if err := c.Delete(context.Background(), "no-existe"); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Errorf("Delete: err = %v, want ErrNotFound", err)
	}
}

func TestPersistenceDownstream(t *testing.T) {
	s := stubs.NewPersistenceStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()
	c := clients.NewPersistence(s.URL)

	if _, err := c.Create(context.Background(), dto.PersistCreateRequest{}); !errors.Is(err, orchestrator.ErrDownstream) {
		t.Errorf("Create: err = %v, want ErrDownstream", err)
	}
}

func strPtr(s string) *string { return &s }
