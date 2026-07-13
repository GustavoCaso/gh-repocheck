package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// StubResponse is a canned reply for one path in a Stub.
type StubResponse struct {
	Status int    // default 200
	Body   string // JSON
}

// RecordedRequest captures a mutating call for assertions.
type RecordedRequest struct {
	Method string
	Path   string
	Body   string
}

// Stub implements Client from canned responses and records mutations.
type Stub struct {
	// Responses maps "METHOD path" (e.g. "GET repos/o/r") to a reply.
	Responses map[string]StubResponse
	Requests  []RecordedRequest
}

var _ Client = (*Stub)(nil)

// HTTPError mirrors go-gh's api.HTTPError shape for status-code checks.
type HTTPError struct {
	StatusCode int
	Path       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Path)
}

// roundTrip performs the shared lookup and recording logic. It returns the
// effective status code and the canned body; found reports whether the path
// had a canned response.
func (s *Stub) roundTrip(method, path string, body io.Reader) (int, string, bool) {
	if method != http.MethodGet && body != nil {
		b, _ := io.ReadAll(body)
		s.Requests = append(s.Requests, RecordedRequest{Method: method, Path: path, Body: string(b)})
	} else if method != http.MethodGet {
		s.Requests = append(s.Requests, RecordedRequest{Method: method, Path: path})
	}
	resp, ok := s.Responses[method+" "+path]
	if !ok {
		return http.StatusNotFound, "", false
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	return status, resp.Body, true
}

// do implements the error-returning methods (Get/Post/Patch/Put).
func (s *Stub) do(method, path string, body io.Reader, response any) error {
	status, respBody, found := s.roundTrip(method, path, body)
	if !found {
		return &HTTPError{StatusCode: http.StatusNotFound, Path: path}
	}
	if status >= http.StatusBadRequest {
		return &HTTPError{StatusCode: status, Path: path}
	}
	if response != nil && respBody != "" {
		return json.Unmarshal([]byte(respBody), response)
	}
	return nil
}

func (s *Stub) Get(_ context.Context, path string, response any) error {
	return s.do(http.MethodGet, path, nil, response)
}

func (s *Stub) Post(_ context.Context, path string, body io.Reader, response any) error {
	return s.do(http.MethodPost, path, body, response)
}

func (s *Stub) Patch(_ context.Context, path string, body io.Reader, response any) error {
	return s.do(http.MethodPatch, path, body, response)
}

func (s *Stub) Put(_ context.Context, path string, body io.Reader, response any) error {
	return s.do(http.MethodPut, path, body, response)
}
