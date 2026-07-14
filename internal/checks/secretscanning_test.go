package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func TestSecretScanningPass(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: `{"security_and_analysis":{
			"secret_scanning":{"status":"enabled"},
			"secret_scanning_push_protection":{"status":"enabled"}}}`},
	}}
	if res := run(t, &SecretScanning{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestSecretScanningFailWhenDisabled(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: `{"security_and_analysis":{
			"secret_scanning":{"status":"disabled"},
			"secret_scanning_push_protection":{"status":"disabled"}}}`},
	}}
	res := run(t, &SecretScanning{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 2 {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestSecretScanningSkipWithoutGHAS(t *testing.T) {
	// security_and_analysis absent entirely: private repo without GHAS
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: `{}`},
	}}
	privateRepo := testRepo()
	privateRepo.Private = true
	res := (&SecretScanning{}).Run(context.Background(), stub, privateRepo, policy.Defaults())
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.Status != check.Skip {
		t.Errorf("status = %v, want Skip", res.Status)
	}
}

func TestSecretScanningFix(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"PATCH repos/o/r": {Body: `{}`},
	}}
	if err := (&SecretScanning{}).Fix(context.Background(), stub, testRepo(), policy.Defaults()); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 {
		t.Fatalf("requests = %v", stub.Requests)
	}
	body := stub.Requests[0].Body
	for _, want := range []string{`"secret_scanning":{"status":"enabled"}`, `"secret_scanning_push_protection":{"status":"enabled"}`} {
		if !strings.Contains(body, want) {
			t.Errorf("PATCH body missing %s: %s", want, body)
		}
	}
}
