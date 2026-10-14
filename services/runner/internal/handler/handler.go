package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/kinonoja/gobot/services/runner/internal/sandbox"
	"github.com/kinonoja/gobot/services/runner/internal/testparse"
	"github.com/kinonoja/gobot/services/runner/internal/types"
	"github.com/kinonoja/gobot/services/runner/internal/validation"
)

type Handler struct {
	sem chan struct{}
	mu  sync.Mutex
}

func New(maxConcurrent int) *Handler {
	return &Handler{
		sem: make(chan struct{}, maxConcurrent),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /run", h.handleRun)
	mux.HandleFunc("GET /healthz", h.handleHealthz)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	select {
	case h.sem <- struct{}{}:
		defer func() { <-h.sem }()
	default:
		http.Error(w, `{"error":"too many concurrent requests"}`, http.StatusTooManyRequests)
		return
	}

	var req types.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := validation.Validate(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := sandbox.Execute(r.Context(), &req)

	if req.Mode == "test" {
		resp.Tests = testparse.Parse(resp.Stdout)
		resp.MissingTests = testparse.Missing(req.ExpectedTests, resp.Tests)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
