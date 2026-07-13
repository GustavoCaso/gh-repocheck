package checks

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func TestRulesetsPass(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":1,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"rules":[{"type":"deletion"},{"type":"non_fast_forward"}]}`},
	}}
	if res := run(t, &Rulesets{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenNoRulesets(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[]`},
	}}
	res := run(t, &Rulesets{}, stub)
	if res.Status != check.Fail || res.Findings[0].FixHint == "" {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenRulesMissing(t *testing.T) {
	// ruleset targets default branch but only blocks deletion
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":1,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"rules":[{"type":"deletion"}]}`},
	}}
	res := run(t, &Rulesets{}, stub)
	if res.Status != check.Fail {
		t.Errorf("status = %v", res.Status)
	}
	if !strings.Contains(res.Findings[0].Message, "non_fast_forward") {
		t.Errorf("finding should name the missing rule: %v", res.Findings)
	}
}

func TestRulesetsIgnoresInactiveAndNonDefaultBranch(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[
			{"id":1,"target":"branch","enforcement":"disabled"},
			{"id":2,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/2": {Body: `{
			"id":2,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/release"],"exclude":[]}},
			"rules":[{"type":"deletion"},{"type":"non_fast_forward"}]}`},
	}}
	if res := run(t, &Rulesets{}, stub); res.Status != check.Fail {
		t.Errorf("status = %v: disabled/off-target rulesets must not count", res.Status)
	}
}

func TestRulesetsSkipsOrgInheritedRulesets(t *testing.T) {
	// Org-inherited rulesets show up in the repo list, but their detail
	// endpoint 404s at the repo level (unstubbed here). They must be
	// skipped, not treated as an error.
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":7,"target":"branch","enforcement":"active"}]`},
	}}
	res := run(t, &Rulesets{}, stub)
	if res.Status != check.Fail {
		t.Errorf("status = %v: uninspectable org ruleset should not count as covering", res.Status)
	}
}

func TestRulesetsFailWhenPullRequestParamsMismatch(t *testing.T) {
	// pull_request rule exists but its parameters fall short of the policy.
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":1,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"rules":[
				{"type":"deletion"},{"type":"non_fast_forward"},
				{"type":"pull_request","parameters":{
					"required_approving_review_count":1,
					"dismiss_stale_reviews_on_push":false,
					"require_code_owner_review":false,
					"require_last_push_approval":false,
					"required_review_thread_resolution":false,
					"allowed_merge_methods":["merge","squash","rebase"]}}]}`},
	}}
	pol := policy.Defaults()
	pol.Checks.Rulesets.Rules.RequirePR = true
	pol.Checks.Rulesets.Rules.RequiredApprovals = 2
	pol.Checks.Rulesets.Rules.RequireCodeOwnerReview = true
	pol.Checks.Rulesets.Rules.AllowedMergeMethods = []policy.MergeType{policy.SquashMethod}
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail {
		t.Fatalf("status = %v, findings = %v", res.Status, res.Findings)
	}
	msg := res.Findings[0].Message
	for _, want := range []string{
		"required_approving_review_count is 1, want at least 2",
		"require_code_owner_review is disabled",
		`merge method "merge" allowed but not permitted by policy`,
		`merge method "rebase" allowed but not permitted by policy`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("finding missing %q: %s", want, msg)
		}
	}
}

func TestRulesetsPassWhenParamsSatisfyPolicy(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":1,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"rules":[
				{"type":"deletion"},{"type":"non_fast_forward"},
				{"type":"required_signatures"},{"type":"required_linear_history"},
				{"type":"pull_request","parameters":{
					"required_approving_review_count":2,
					"dismiss_stale_reviews_on_push":true,
					"require_code_owner_review":true,
					"require_last_push_approval":false,
					"required_review_thread_resolution":false,
					"allowed_merge_methods":["squash"]}},
				{"type":"required_status_checks","parameters":{
					"strict_required_status_checks_policy":true,
					"required_status_checks":[{"context":"ci/test"}]}}]}`},
	}}
	pol := policy.Defaults()
	rules := &pol.Checks.Rulesets.Rules
	rules.RequireSignatures = true
	rules.RequireLinearHistory = true
	rules.RequirePR = true
	rules.RequiredApprovals = 2
	rules.DismissStaleReviews = true
	rules.RequireCodeOwnerReview = true
	rules.AllowedMergeMethods = []policy.MergeType{policy.SquashMethod}
	rules.RequiredStatusChecks = []string{"ci/test"}
	rules.StrictStatusChecks = true
	if res := runWithPolicy(t, &Rulesets{}, stub, pol); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenStatusCheckMissing(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100": {Body: `[{"id":1,"target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"rules":[
				{"type":"deletion"},{"type":"non_fast_forward"},
				{"type":"required_status_checks","parameters":{
					"strict_required_status_checks_policy":false,
					"required_status_checks":[{"context":"ci/test"}]}}]}`},
	}}
	pol := policy.Defaults()
	pol.Checks.Rulesets.Rules.RequiredStatusChecks = []string{"ci/test", "ci/lint"}
	pol.Checks.Rulesets.Rules.StrictStatusChecks = true
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail {
		t.Fatalf("status = %v", res.Status)
	}
	msg := res.Findings[0].Message
	for _, want := range []string{`status check "ci/lint" not required`, "strict_required_status_checks_policy is disabled"} {
		if !strings.Contains(msg, want) {
			t.Errorf("finding missing %q: %s", want, msg)
		}
	}
}

func TestRulesetsFix(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"POST repos/o/r/rulesets": {Body: `{"id":9}`},
	}}
	pol := policy.Defaults()
	rules := &pol.Checks.Rulesets.Rules
	rules.RequireSignatures = true
	rules.RequireLinearHistory = true
	rules.RequirePR = true
	rules.RequiredApprovals = 1
	rules.DismissStaleReviews = true
	rules.AllowedMergeMethods = []policy.MergeType{policy.SquashMethod}
	rules.RequiredStatusChecks = []string{"ci/test"}
	rules.StrictStatusChecks = true
	if err := (&Rulesets{}).Fix(context.Background(), stub, testRepo(), pol); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 {
		t.Fatalf("requests = %v", stub.Requests)
	}
	body := stub.Requests[0].Body
	for _, want := range []string{
		`"target":"branch"`, `"enforcement":"active"`, `"~DEFAULT_BRANCH"`,
		`"type":"deletion"`, `"type":"non_fast_forward"`, `"type":"pull_request"`,
		`"type":"required_signatures"`, `"type":"required_linear_history"`,
		`"required_approving_review_count":1`,
		`"dismiss_stale_reviews_on_push":true`,
		`"allowed_merge_methods":["squash"]`,
		`"type":"required_status_checks"`,
		`"required_status_checks":[{"context":"ci/test"}]`,
		`"strict_required_status_checks_policy":true`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("POST body missing %s\nbody: %s", want, body)
		}
	}
}
