package runnerclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus string
		wantErr    error
	}{
		{name: "success", status: http.StatusOK, body: `{"status":"ok","missing_tests":[]}`, wantStatus: "ok"},
		{name: "rate limited", status: http.StatusTooManyRequests, wantErr: ErrRateLimited},
		{name: "server error", status: http.StatusBadGateway, body: "unavailable", wantErr: errNon200{}},
		{name: "invalid JSON", status: http.StatusOK, body: "{", wantErr: errDecode{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/run" {
					t.Errorf("request = %s %s, want POST /run", r.Method, r.URL.Path)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			response, err := New(server.URL, time.Second).Run(context.Background(), Request{Mode: "test"})
			if test.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}
				if test.wantErr == ErrRateLimited && !errors.Is(err, ErrRateLimited) {
					t.Fatalf("error = %v, want ErrRateLimited", err)
				}
				if _, ok := test.wantErr.(errNon200); ok && err.Error() == "" {
					t.Fatal("expected non-200 error message")
				}
				if _, ok := test.wantErr.(errDecode); ok && err.Error() == "" {
					t.Fatal("expected decode error message")
				}
				return
			}
			if err != nil || response.Status != test.wantStatus {
				t.Fatalf("Run = (%+v, %v), want status %q", response, err, test.wantStatus)
			}
		})
	}
}

func TestRunTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()
	_, err := New(server.URL, 10*time.Millisecond).Run(context.Background(), Request{})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("error = %v, want timeout", err)
	}
}

type errNon200 struct{}

func (errNon200) Error() string { return "non-200" }

type errDecode struct{}

func (errDecode) Error() string { return "decode" }
