package runnerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var ErrRateLimited = errors.New("runner rate limited the request")
var ErrTimeout = errors.New("runner request timed out")

type Request struct {
	Files         map[string]string `json:"files"`
	GraderFiles   map[string]string `json:"grader_files,omitempty"`
	ExpectedTests []string          `json:"expected_tests,omitempty"`
	Mode          string            `json:"mode"`
	TimeoutMs     int               `json:"timeout_ms"`
}

type TestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Output string `json:"output"`
}

type Response struct {
	Status       string       `json:"status"`
	Stdout       string       `json:"stdout"`
	Stderr       string       `json:"stderr"`
	DurationMs   int64        `json:"duration_ms"`
	Tests        []TestResult `json:"tests"`
	MissingTests []string     `json:"missing_tests"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: timeout}}
}

func (c *Client) Run(ctx context.Context, payload Request) (Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("encode runner request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/run", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create runner request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return Response{}, fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return Response{}, fmt.Errorf("runner unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return Response{}, ErrRateLimited
	}
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return Response{}, fmt.Errorf("runner returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var result Response
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Response{}, fmt.Errorf("decode runner response: %w", err)
	}
	return result, nil
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
