package githubapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestStubGetHit(t *testing.T) {
	s := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r": {Body: `{"name":"r","private":true}`},
	}}
	var got struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	}
	if err := s.Get(context.Background(), "repos/o/r", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "r" || !got.Private {
		t.Errorf("unmarshaled %+v", got)
	}
	if len(s.Requests) != 0 {
		t.Errorf("GET should not be recorded, got %v", s.Requests)
	}
}

func TestStubGetMiss(t *testing.T) {
	s := &Stub{}
	err := s.Get(context.Background(), "repos/o/r", nil)
	if err == nil {
		t.Fatal("expected error on missing path")
	}
	if got := StatusCode(err); got != 404 {
		t.Errorf("StatusCode(err) = %d, want 404", got)
	}
}

func TestStubGetErrorStatus(t *testing.T) {
	s := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r": {Status: 403, Body: `{"message":"forbidden"}`},
	}}
	err := s.Get(context.Background(), "repos/o/r", nil)
	if got := StatusCode(err); got != 403 {
		t.Errorf("StatusCode(err) = %d, want 403", got)
	}
}

func TestStubPatchRecords(t *testing.T) {
	s := &Stub{Responses: map[string]StubResponse{
		"PATCH repos/o/r": {Body: `{}`},
	}}
	err := s.Patch(context.Background(), "repos/o/r", strings.NewReader(`{"has_wiki":false}`), nil)
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if len(s.Requests) != 1 {
		t.Fatalf("Requests = %v, want 1 entry", s.Requests)
	}
	r := s.Requests[0]
	if r.Method != http.MethodPatch || r.Path != "repos/o/r" || r.Body != `{"has_wiki":false}` {
		t.Errorf("recorded %+v", r)
	}
}

func TestStubGetNoContent(t *testing.T) {
	// The "enabled" probe pattern: a 204 entry with no body must return nil
	// error and leave the target untouched.
	s := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/vulnerability-alerts": {Status: 204},
	}}
	got := struct {
		Name string `json:"name"`
	}{Name: "untouched"}
	if err := s.Get(context.Background(), "repos/o/r/vulnerability-alerts", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "untouched" {
		t.Errorf("target modified on 204: %+v", got)
	}
}
