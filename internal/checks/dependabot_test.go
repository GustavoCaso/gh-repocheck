package checks

import (
	"context"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// GET vulnerability-alerts: 204 = enabled, 404 = disabled.
// GET automated-security-fixes: {"enabled": bool}.

func dependabotStub(alerts int, secFixes string) *githubapi.Stub {
	return &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/vulnerability-alerts":     {Status: alerts},
		"GET repos/o/r/automated-security-fixes": {Body: secFixes},
	}}
}

func TestDependabotPass(t *testing.T) {
	stub := dependabotStub(204, `{"enabled":true}`)
	if res := run(t, &Dependabot{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestDependabotFailWhenAlertsOff(t *testing.T) {
	stub := dependabotStub(404, `{"enabled":false}`)
	res := run(t, &Dependabot{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 2 {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestDependabotAlertsProbeErrorSurfaces(t *testing.T) {
	// A 403 (e.g. missing scope) must surface as an error, not as "disabled".
	stub := dependabotStub(403, `{"enabled":true}`)
	if _, err := (&Dependabot{}).Run(context.Background(), stub, testRepo(), policy.Defaults()); err == nil {
		t.Fatal("expected error when alerts probe returns 403")
	}
}

func TestDependabotFix(t *testing.T) {
	stub := dependabotStub(404, `{"enabled":false}`)
	stub.Responses["PUT repos/o/r/vulnerability-alerts"] = githubapi.StubResponse{Status: 204}
	stub.Responses["PUT repos/o/r/automated-security-fixes"] = githubapi.StubResponse{Status: 204}
	if err := (&Dependabot{}).Fix(context.Background(), stub, testRepo(), policy.Defaults()); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 2 {
		t.Fatalf("expected 2 PUTs, got %v", stub.Requests)
	}
}
