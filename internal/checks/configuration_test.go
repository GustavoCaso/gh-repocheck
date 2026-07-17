package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func configurationPolicy() policy.Policy {
	pol := policy.Defaults()
	pol.Checks.Configuration.Enabled = true
	return pol
}

func TestConfigurationPass(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: `{
			"has_issues": true,
			"has_projects": true,
			"has_wiki": true,
			"allow_squash_merge": true,
			"allow_merge_commit": false,
			"allow_rebase_merge": true,
			"allow_auto_merge": false,
			"delete_branch_on_merge": false,
			"web_commit_signoff_required": false}`},
	}}
	res := runWithPolicy(t, &Configuration{}, stub, configurationPolicy())
	if res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestConfigurationFailReportsEveryMismatch(t *testing.T) {
	// has_issues and allow_squash_merge differ from the default policy;
	// both must show up as findings, not just the first.
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r": {Body: `{
			"has_issues": false,
			"has_projects": true,
			"has_wiki": true,
			"allow_squash_merge": false,
			"allow_merge_commit": false,
			"allow_rebase_merge": true,
			"allow_auto_merge": false,
			"delete_branch_on_merge": false,
			"web_commit_signoff_required": false}`},
	}}
	res := runWithPolicy(t, &Configuration{}, stub, configurationPolicy())
	if res.Status != check.Fail || len(res.Findings) != 2 {
		t.Fatalf("status = %v, findings = %v", res.Status, res.Findings)
	}
	for i, want := range []string{"has_issues", "allow_squash_merge"} {
		if !strings.Contains(res.Findings[i].Message, want) {
			t.Errorf("findings[%d] = %q, want mention of %s", i, res.Findings[i].Message, want)
		}
		if res.Findings[i].FixHint == "" {
			t.Errorf("findings[%d] missing FixHint", i)
		}
	}
}

func TestConfigurationDisabledByPolicy(t *testing.T) {
	if (&Configuration{}).Enabled(policy.Defaults()) {
		t.Error("configuration check should default disabled")
	}
	if !(&Configuration{}).Enabled(configurationPolicy()) {
		t.Error("configuration check should be enabled by policy")
	}
}

func TestConfigurationFix(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"PATCH repos/o/r": {Body: `{}`},
	}}
	pol := configurationPolicy()
	pol.Checks.Configuration.AllowAutoMerge = true
	if err := (&Configuration{}).Fix(context.Background(), stub, testRepo(), pol); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 {
		t.Fatalf("requests = %v", stub.Requests)
	}
	body := stub.Requests[0].Body
	for _, want := range []string{
		`"has_issues":true`,
		`"allow_squash_merge":true`,
		`"allow_merge_commit":false`,
		`"allow_auto_merge":true`,
		`"web_commit_signoff_required":false`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("PATCH body missing %s: %s", want, body)
		}
	}
}
