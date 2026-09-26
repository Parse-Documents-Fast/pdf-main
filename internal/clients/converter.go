package clients

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

// Converter is the HTTP client for pdf-converter (download-only, ADR-0005).
type Converter struct {
	hc *httpClient
}

// NewConverter builds a converter client against baseURL.
func NewConverter(baseURL string) *Converter {
	return &Converter{hc: newHTTPClient("converter", baseURL)}
}

// Convert turns Markdown into PDF bytes. Any non-200 response maps to
// orchestrator.ErrDownstream.
func (c *Converter) Convert(ctx context.Context, content string) (dto.ConvertResponse, error) {
	body, err := json.Marshal(dto.ConvertRequest{Content: content})
	if err != nil {
		return dto.ConvertResponse{}, err
	}

	status, data, err := c.hc.do(ctx, http.MethodPost, dto.PathConverterConvert, body)
	if err != nil {
		return dto.ConvertResponse{}, err
	}

	if status != http.StatusOK {
		return dto.ConvertResponse{}, orchestrator.ErrDownstream
	}

	var v dto.ConvertResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return dto.ConvertResponse{}, orchestrator.ErrDownstream
	}
	return v, nil
}
