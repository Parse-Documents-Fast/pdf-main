package stubs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// PersistenceStub is an in-memory fake of pdf-persistence. It stores records
// keyed by an auto-incrementing ID and supports the full CRUD surface used by
// pdf-main: create, get, find-by-checksum, list, update and delete.
type PersistenceStub struct {
	*httptest.Server

	mu      sync.Mutex
	records map[string]dto.PersistRecord
	seq     int

	// Delay, when > 0, is slept before responding (to simulate timeouts).
	Delay time.Duration
	// FailStatus, when != 0, makes every request respond with that status.
	FailStatus int
}

// NewPersistenceStub starts a persistence stub on a random port. Callers must
// Close it when done.
func NewPersistenceStub() *PersistenceStub {
	s := &PersistenceStub{records: make(map[string]dto.PersistRecord)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+dto.PathPersistDocuments, s.create)
	mux.HandleFunc("GET "+dto.PathPersistDocuments, s.list)
	mux.HandleFunc("GET "+dto.PathPersistFindByChecksum, s.findByChecksum)
	mux.HandleFunc("GET "+dto.PathPersistDocuments+"/{id}", s.get)
	mux.HandleFunc("PATCH "+dto.PathPersistDocuments+"/{id}", s.update)
	mux.HandleFunc("DELETE "+dto.PathPersistDocuments+"/{id}", s.del)

	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Delay > 0 {
			time.Sleep(s.Delay)
		}
		if s.FailStatus != 0 {
			problem.WriteProblem(w, s.FailStatus, "downstream failure", "forced failure", r.URL.Path)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	return s
}

// Reset clears all stored records, useful between test cases.
func (s *PersistenceStub) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]dto.PersistRecord)
}

func (s *PersistenceStub) create(w http.ResponseWriter, r *http.Request) {
	var req dto.PersistCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, "Invalid request", err.Error(), r.URL.Path)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	rec := dto.PersistRecord{
		ID:             strconv.Itoa(s.seq),
		Title:          req.Title,
		OriginalFormat: req.OriginalFormat,
		Checksum:       req.Checksum,
		Status:         req.Status,
		Content:        req.Content,
		Error:          nil,
		CreatedAt:      time.Now().UTC(),
	}
	s.records[rec.ID] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (s *PersistenceStub) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.records))
	for id := range s.records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, _ := strconv.Atoi(ids[i])
		b, _ := strconv.Atoi(ids[j])
		return a < b
	})

	out := make([]dto.PersistRecord, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.records[id])
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *PersistenceStub) findByChecksum(w http.ResponseWriter, r *http.Request) {
	checksum := r.URL.Query().Get("checksum")

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.records {
		if rec.Checksum == checksum {
			writeJSON(w, http.StatusOK, rec)
			return
		}
	}
	problem.WriteProblem(w, http.StatusNotFound, "Not found", "no document with checksum "+checksum, r.URL.Path)
}

func (s *PersistenceStub) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "no document with id "+id, r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *PersistenceStub) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req dto.PersistUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, "Invalid request", err.Error(), r.URL.Path)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "no document with id "+id, r.URL.Path)
		return
	}
	rec.Status = req.Status
	if req.Content != nil {
		rec.Content = req.Content
	}
	if req.Error != nil {
		rec.Error = req.Error
	}
	s.records[id] = rec
	writeJSON(w, http.StatusOK, rec)
}

func (s *PersistenceStub) del(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		problem.WriteProblem(w, http.StatusNotFound, "Not found", "no document with id "+id, r.URL.Path)
		return
	}
	delete(s.records, id)
	w.WriteHeader(http.StatusNoContent)
}
