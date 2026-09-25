package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
)

// Persistence is the HTTP client for pdf-persistance.
type Persistence struct {
	hc *httpClient
}

// NewPersistence builds a persistence client against baseURL.
func NewPersistence(baseURL string) *Persistence {
	return &Persistence{hc: newHTTPClient("persistence", baseURL)}
}

// Create stores a new document. 409 maps to orchestrator.ErrDuplicate.
func (c *Persistence) Create(ctx context.Context, req dto.PersistCreateRequest) (dto.PersistRecord, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return dto.PersistRecord{}, err
	}

	status, data, err := c.hc.do(ctx, http.MethodPost, dto.PathPersistDocuments, body)
	if err != nil {
		return dto.PersistRecord{}, err
	}

	switch status {
	case http.StatusCreated:
		var r dto.PersistRecord
		if err := json.Unmarshal(data, &r); err != nil {
			return dto.PersistRecord{}, orchestrator.ErrDownstream
		}
		return r, nil
	case http.StatusConflict:
		return dto.PersistRecord{}, orchestrator.ErrDuplicate
	default:
		return dto.PersistRecord{}, orchestrator.ErrDownstream
	}
}

// FindByChecksum looks up a document by checksum. It returns nil (no error)
// when no document matches, to allow duplicate detection.
func (c *Persistence) FindByChecksum(ctx context.Context, checksum string) (*dto.PersistRecord, error) {
	path := dto.PathPersistFindByChecksum + "?checksum=" + url.QueryEscape(checksum)

	status, data, err := c.hc.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusOK:
		var r dto.PersistRecord
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, orchestrator.ErrDownstream
		}
		return &r, nil
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, orchestrator.ErrDownstream
	}
}

// Get returns a document by ID. 404 maps to orchestrator.ErrNotFound.
func (c *Persistence) Get(ctx context.Context, id string) (dto.PersistRecord, error) {
	status, data, err := c.hc.do(ctx, http.MethodGet, c.documentPath(id), nil)
	if err != nil {
		return dto.PersistRecord{}, err
	}

	switch status {
	case http.StatusOK:
		var r dto.PersistRecord
		if err := json.Unmarshal(data, &r); err != nil {
			return dto.PersistRecord{}, orchestrator.ErrDownstream
		}
		return r, nil
	case http.StatusNotFound:
		return dto.PersistRecord{}, orchestrator.ErrNotFound
	default:
		return dto.PersistRecord{}, orchestrator.ErrDownstream
	}
}

// List returns all documents.
func (c *Persistence) List(ctx context.Context) ([]dto.PersistRecord, error) {
	status, data, err := c.hc.do(ctx, http.MethodGet, dto.PathPersistDocuments, nil)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, orchestrator.ErrDownstream
	}

	var list []dto.PersistRecord
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, orchestrator.ErrDownstream
	}
	return list, nil
}

// Update patches a document. 404 maps to orchestrator.ErrNotFound.
func (c *Persistence) Update(ctx context.Context, id string, req dto.PersistUpdateRequest) (dto.PersistRecord, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return dto.PersistRecord{}, err
	}

	status, data, err := c.hc.do(ctx, http.MethodPatch, c.documentPath(id), body)
	if err != nil {
		return dto.PersistRecord{}, err
	}

	switch status {
	case http.StatusOK:
		var r dto.PersistRecord
		if err := json.Unmarshal(data, &r); err != nil {
			return dto.PersistRecord{}, orchestrator.ErrDownstream
		}
		return r, nil
	case http.StatusNotFound:
		return dto.PersistRecord{}, orchestrator.ErrNotFound
	default:
		return dto.PersistRecord{}, orchestrator.ErrDownstream
	}
}

// Delete removes a document. 404 maps to orchestrator.ErrNotFound.
func (c *Persistence) Delete(ctx context.Context, id string) error {
	status, _, err := c.hc.do(ctx, http.MethodDelete, c.documentPath(id), nil)
	if err != nil {
		return err
	}

	switch status {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return orchestrator.ErrNotFound
	default:
		return orchestrator.ErrDownstream
	}
}

// documentPath builds the /documents/{id} path, escaping the id.
func (c *Persistence) documentPath(id string) string {
	return dto.PathPersistDocuments + "/" + url.PathEscape(id)
}

// Compile-time assertions that the clients satisfy the orchestrator ports.
var (
	_ orchestrator.Validator   = (*Validator)(nil)
	_ orchestrator.Converter   = (*Converter)(nil)
	_ orchestrator.Persistence = (*Persistence)(nil)
)
