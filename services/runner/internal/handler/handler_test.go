package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	h := New(5)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want %q", body["status"], "ok")
	}
}

func TestRunValidation(t *testing.T) {
	h := New(5)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{
			name:     "invalid json",
			body:     "{bad",
			wantCode: http.StatusBadRequest,
			wantErr:  "invalid JSON",
		},
		{
			name:     "invalid mode",
			body:     `{"files":{"main.go":"package main"},"mode":"exec","timeout_ms":5000}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "invalid mode",
		},
		{
			name:     "no files",
			body:     `{"files":{},"mode":"run","timeout_ms":5000}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "no files",
		},
		{
			name:     "path traversal",
			body:     `{"files":{"../bad.go":"package main"},"mode":"run","timeout_ms":5000}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "contains \"..\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantCode)
			}

			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !strings.Contains(body["error"], tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", body["error"], tt.wantErr)
			}
		})
	}
}

func TestRunConcurrencyLimit(t *testing.T) {
	h := New(0)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"files":{"main.go":"package main\nfunc main(){}"},"mode":"run","timeout_ms":5000}`
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}
