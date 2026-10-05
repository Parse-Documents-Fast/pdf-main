package clients_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/test/stubs"
)

func TestExtractorExtractOK(t *testing.T) {
	s := stubs.NewExtractorStub()
	defer s.Close()

	c := clients.NewExtractor(s.URL)
	got, err := c.Extract(context.Background(), []byte("%PDF-1.4\nfake"))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got.PageCount != s.PageCount {
		t.Errorf("PageCount = %d, want %d", got.PageCount, s.PageCount)
	}
	if got.Content != s.Content {
		t.Errorf("Content = %q, want %q", got.Content, s.Content)
	}
}

func TestExtractorExtractDownstream(t *testing.T) {
	s := stubs.NewExtractorStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()

	c := clients.NewExtractor(s.URL)
	_, err := c.Extract(context.Background(), []byte("%PDF-1.4"))
	if !errors.Is(err, orchestrator.ErrDownstream) {
		t.Fatalf("err = %v, want ErrDownstream", err)
	}
}

func TestExtractorExtractDownstreamCarriesService(t *testing.T) {
	s := stubs.NewExtractorStub()
	s.FailStatus = http.StatusInternalServerError
	defer s.Close()

	c := clients.NewExtractor(s.URL)
	_, err := c.Extract(context.Background(), []byte("%PDF-1.4"))

	var de orchestrator.DownstreamError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, want DownstreamError", err)
	}
	if de.Service != "extractor" {
		t.Errorf("Service = %q, want extractor", de.Service)
	}
}
