package checks

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// rulesetsPolicy enables the rulesets check with the given named rule sets
// for one branch.
//
//nolint:unparam // for now I use only main
func rulesetsPolicy(branch string, rules ...policy.RulesetRules) policy.Policy {
	pol := policy.Defaults()
	pol.Checks.Rulesets.Enabled = true
	pol.Checks.Rulesets.Rules = map[string][]policy.RulesetRules{branch: rules}
	return pol
}

func TestRulesetsPassWhenNamedRulesetMatches(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"rules":[{"type":"deletion"},{"type":"non_fast_forward"}]}`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
	})
	if res := runWithPolicy(t, &Rulesets{}, stub, pol); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenNamedRulesetMissing(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Body: `[]`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{Name: "protect", BlockDeletion: true})
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail || len(res.Findings) == 0 || res.Findings[0].FixHint == "" {
		t.Fatalf("status = %v, findings = %v", res.Status, res.Findings)
	}
	msg := res.Findings[0].Message
	if !strings.Contains(msg, "protect") || !strings.Contains(msg, "main") {
		t.Errorf("finding should name the ruleset and branch: %s", msg)
	}
}

func TestRulesetsFailWhenRulesMissing(t *testing.T) {
	// named ruleset exists but only blocks deletion
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"rules":[{"type":"deletion"}]}`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
	})
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail {
		t.Fatalf("status = %v", res.Status)
	}
	if !strings.Contains(res.Findings[0].Message, "non_fast_forward") {
		t.Errorf("finding should name the missing rule: %v", res.Findings)
	}
}

func TestRulesetsFailWhenNotActiveOrWrongBranch(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Body: `[
			{"id":1,"name":"protect","target":"branch","enforcement":"disabled"},
			{"id":2,"name":"release-protect","target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"disabled",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"rules":[{"type":"deletion"}]}`},
		"GET repos/o/r/rulesets/2": {Body: `{
			"id":2,"name":"release-protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/release"],"exclude":[]}},
			"rules":[{"type":"deletion"}]}`},
	}}
	pol := rulesetsPolicy("main",
		policy.RulesetRules{Name: "protect", BlockDeletion: true},
		policy.RulesetRules{Name: "release-protect", BlockDeletion: true},
	)
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail || len(res.Findings) != 2 {
		t.Fatalf("status = %v, findings = %v", res.Status, res.Findings)
	}
	joined := res.Findings[0].Message + res.Findings[1].Message
	if !strings.Contains(joined, "enforcement") {
		t.Errorf("finding should flag inactive enforcement: %s", joined)
	}
	if !strings.Contains(joined, "refs/heads/main") {
		t.Errorf("finding should flag branch not covered: %s", joined)
	}
}

func TestRulesetsSkipsUninspectableOrgRuleset(t *testing.T) {
	// Org-inherited rulesets show up in the repo list, but their detail
	// endpoint 404s at the repo level (unstubbed here). A name match on
	// one must not fail the check — it exists, it just can't be inspected.
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":7,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{Name: "protect", BlockDeletion: true})
	if res := runWithPolicy(t, &Rulesets{}, stub, pol); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenPullRequestParamsMismatch(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
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
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
		RequirePR: true, RequiredApprovals: 2, RequireCodeOwnerReview: true,
		AllowedMergeMethods: []policy.MergeType{policy.SquashMethod},
	})
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
	// ~DEFAULT_BRANCH covers the policy branch when it is the default branch.
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
			"bypass_actors":[{"actor_id":5,"actor_type":"RepositoryRole","bypass_mode":"always"}],
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
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
		RequireSignatures: true, RequireLinearHistory: true,
		BypassByAdminRole: true, BypassModeAdmin: policy.AlwaysMode,
		RequirePR: true, RequiredApprovals: 2, DismissStaleReviews: true,
		RequireCodeOwnerReview: true,
		AllowedMergeMethods:    []policy.MergeType{policy.SquashMethod},
		RequiredStatusChecks:   []string{"ci/test"}, StrictStatusChecks: true,
	})
	if res := runWithPolicy(t, &Rulesets{}, stub, pol); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestRulesetsFailWhenStatusCheckMissing(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"rules":[
				{"type":"deletion"},{"type":"non_fast_forward"},
				{"type":"required_status_checks","parameters":{
					"strict_required_status_checks_policy":false,
					"required_status_checks":[{"context":"ci/test"}]}}]}`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
		RequiredStatusChecks: []string{"ci/test", "ci/lint"}, StrictStatusChecks: true,
	})
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

func TestRulesetsFailWhenBypassActorsMismatch(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"bypass_actors":[{"actor_id":4,"actor_type":"RepositoryRole","bypass_mode":"always"}],
			"rules":[{"type":"deletion"}]}`},
	}}
	// Policy wants admin bypass; ruleset instead lets the write role bypass.
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockDeletion: true,
		BypassByAdminRole: true, BypassModeAdmin: policy.AlwaysMode,
	})
	res := runWithPolicy(t, &Rulesets{}, stub, pol)
	if res.Status != check.Fail {
		t.Fatalf("status = %v, findings = %v", res.Status, res.Findings)
	}
	msg := res.Findings[0].Message
	if !strings.Contains(msg, "admin") {
		t.Errorf("finding should flag missing admin bypass: %s", msg)
	}
	if !strings.Contains(msg, "not permitted by policy") {
		t.Errorf("finding should flag the extra write-role bypass: %s", msg)
	}
}

func TestRulesetsFixCreatesMissingRuleset(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Body: `[]`},
		"POST repos/o/r/rulesets":                    {Body: `{"id":9}`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
		RequireSignatures: true, RequireLinearHistory: true,
		BypassByAdminRole: true, BypassModeAdmin: policy.AlwaysMode,
		RequirePR: true, RequiredApprovals: 1, DismissStaleReviews: true,
		AllowedMergeMethods:  []policy.MergeType{policy.SquashMethod},
		RequiredStatusChecks: []string{"ci/test"}, StrictStatusChecks: true,
	})
	if err := (&Rulesets{}).Fix(context.Background(), stub, testRepo(), pol); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 || stub.Requests[0].Method != http.MethodPost {
		t.Fatalf("requests = %v", stub.Requests)
	}
	body := stub.Requests[0].Body
	for _, want := range []string{
		`"name":"protect"`, `"target":"branch"`, `"enforcement":"active"`,
		`"refs/heads/main"`,
		`"type":"deletion"`, `"type":"non_fast_forward"`, `"type":"pull_request"`,
		`"type":"required_signatures"`, `"type":"required_linear_history"`,
		`"required_approving_review_count":1`,
		`"dismiss_stale_reviews_on_push":true`,
		`"allowed_merge_methods":["squash"]`,
		`"type":"required_status_checks"`,
		`"required_status_checks":[{"context":"ci/test"}]`,
		`"strict_required_status_checks_policy":true`,
		`"actor_id":5`, `"actor_type":"RepositoryRole"`, `"bypass_mode":"always"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("POST body missing %s\nbody: %s", want, body)
		}
	}
}

func TestRulesetsFixUpdatesExistingRuleset(t *testing.T) {
	stub := &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":3,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/3": {Body: `{
			"id":3,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
			"rules":[{"type":"deletion"}]}`},
		"PUT repos/o/r/rulesets/3": {Body: `{"id":3}`},
	}}
	pol := rulesetsPolicy("main", policy.RulesetRules{
		Name: "protect", BlockForcePush: true, BlockDeletion: true,
	})
	if err := (&Rulesets{}).Fix(context.Background(), stub, testRepo(), pol); err != nil {
		t.Fatal(err)
	}
	if len(stub.Requests) != 1 || stub.Requests[0].Method != http.MethodPut ||
		stub.Requests[0].Path != "repos/o/r/rulesets/3" {
		t.Fatalf("requests = %v", stub.Requests)
	}
	body := stub.Requests[0].Body
	for _, want := range []string{`"name":"protect"`, `"type":"non_fast_forward"`, `"refs/heads/main"`} {
		if !strings.Contains(body, want) {
			t.Errorf("PUT body missing %s\nbody: %s", want, body)
		}
	}
}
