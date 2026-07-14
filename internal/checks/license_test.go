package checks

import (
	"context"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func licenseStub(body string) *githubapi.Stub {
	return &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: body},
	}}
}

func TestLicensePass(t *testing.T) {
	stub := licenseStub(`{"license":{"spdx_id":"MIT"}}`)
	if res := run(t, &License{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v", res.Status)
	}
}

func TestLicenseFailWhenMissing(t *testing.T) {
	stub := licenseStub(`{"license":null}`)
	res := run(t, &License{}, stub)
	if res.Status != check.Fail {
		t.Errorf("status = %v", res.Status)
	}
	if res.Findings[0].FixHint != "" {
		t.Error("license is not auto-fixable")
	}
}

func TestLicenseFailWhenNotInAllowedList(t *testing.T) {
	stub := licenseStub(`{"license":{"spdx_id":"GPL-3.0"}}`)
	pol := policy.Defaults()
	pol.Checks.License.Allowed = []string{"MIT", "Apache-2.0"}
	res := (&License{}).Run(context.Background(), stub, testRepo(), pol)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.Status != check.Fail {
		t.Errorf("status = %v, want Fail for disallowed license", res.Status)
	}
}

func TestLicenseIsNotFixable(t *testing.T) {
	var c check.Check = &License{}
	if _, ok := c.(check.Fixable); ok {
		t.Error("License must not be Fixable — choosing a license is a human decision")
	}
}
