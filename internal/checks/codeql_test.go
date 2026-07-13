package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func TestCodeQLPass(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/code-scanning/default-setup": {Body: `{"state":"configured","languages":["go"]}`},
	}}
	if res := run(t, &CodeQL{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v", res.Status)
	}
}

func TestCodeQLFailWhenNotConfigured(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/code-scanning/default-setup": {Body: `{"state":"not-configured","languages":["go"]}`},
	}}
	res := run(t, &CodeQL{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 1 || res.Findings[0].FixHint == "" {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestCodeQLSkipOn403(t *testing.T) {
	// 403: code scanning unavailable (private repo without GHAS)
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/code-scanning/default-setup": {Status: 403},
	}}
	if res := run(t, &CodeQL{}, stub); res.Status != check.Skip {
		t.Errorf("status = %v, want Skip", res.Status)
	}
}

func TestCodeQLSkipWhenNoSupportedLanguages(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/code-scanning/default-setup": {Body: `{"state":"not-configured","languages":[]}`},
	}}
	if res := run(t, &CodeQL{}, stub); res.Status != check.Skip {
		t.Errorf("status = %v, want Skip", res.Status)
	}
}

func TestCodeQLFix(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"PATCH repos/o/r/code-scanning/default-setup": {Body: `{}`},
	}}
	if err := (&CodeQL{}).Fix(context.Background(), stub, testRepo, policy.Defaults()); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 || !strings.Contains(stub.Requests[0].Body, `"state":"configured"`) {
		t.Errorf("requests = %v", stub.Requests)
	}
}
