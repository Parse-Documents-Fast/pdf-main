package clients

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

// Extractor is the synchronous HTTP client for pdf-extractor. It backs the
// benchmark-only POST /extract endpoint (ADR-0004 deviation: direct sync call
// instead of the queue).
type Extractor struct {
	hc *httpClient
}

// NewExtractor builds an extractor client against baseURL.
func NewExtractor(baseURL string) *Extractor {
	return &Extractor{hc: newHTTPClient("extractor", baseURL)}
}

// Extract sends a PDF and returns the extracted Markdown and page count. Any
// non-200 response maps to orchestrator.ErrDownstream.
func (c *Extractor) Extract(ctx context.Context, content []byte) (dto.ExtractResponse, error) {
	body, err := json.Marshal(dto.ExtractRequest{ContentBase64: content})
	if err != nil {
		return dto.ExtractResponse{}, err
	}

	status, data, err := c.hc.do(ctx, http.MethodPost, dto.PathExtractorExtract, body)
	if err != nil {
		return dto.ExtractResponse{}, err
	}

	if status != http.StatusOK {
		return dto.ExtractResponse{}, orchestrator.ErrDownstream
	}

	var resp dto.ExtractResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return dto.ExtractResponse{}, orchestrator.ErrDownstream
	}
	return resp, nil
}

var _ orchestrator.Extractor = (*Extractor)(nil)
