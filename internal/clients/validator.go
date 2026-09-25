package clients

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

// Validator is the HTTP client for pdf-validator.
type Validator struct {
	hc *httpClient
}

// NewValidator builds a validator client against baseURL.
func NewValidator(baseURL string) *Validator {
	return &Validator{hc: newHTTPClient("validator", baseURL)}
}

// Validate classifies content and returns its format and checksum. A 400 maps
// to orchestrator.ErrInvalid; transport/5xx map to orchestrator.ErrDownstream.
func (c *Validator) Validate(ctx context.Context, content []byte, filename string) (dto.ValidateResponse, error) {
	req := dto.ValidateRequest{Filename: filename, ContentBase64: content}
	body, err := json.Marshal(req)
	if err != nil {
		return dto.ValidateResponse{}, err
	}

	status, data, err := c.hc.do(ctx, http.MethodPost, dto.PathValidatorValidate, body)
	if err != nil {
		return dto.ValidateResponse{}, err
	}

	switch status {
	case http.StatusOK:
		var v dto.ValidateResponse
		if err := json.Unmarshal(data, &v); err != nil {
			return dto.ValidateResponse{}, orchestrator.ErrDownstream
		}
		return v, nil
	case http.StatusBadRequest:
		return dto.ValidateResponse{}, orchestrator.ErrInvalid
	default:
		return dto.ValidateResponse{}, orchestrator.ErrDownstream
	}
}
