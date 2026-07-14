package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type Rulesets struct{}

var _ check.Fixable = (*Rulesets)(nil)

func (r *Rulesets) ID() string          { return "rulesets" }
func (r *Rulesets) Description() string { return "A ruleset protects the default branch" }

func (r *Rulesets) Enabled(pol policy.Policy) bool { return pol.Checks.Rulesets.Enabled }

type rulesetSummary struct {
	ID          int64  `json:"id"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
}

type ruleset struct {
	rulesetSummary

	Conditions struct {
		RefName struct {
			Include []string `json:"include"`
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []struct {
		Type       string          `json:"type"`
		Parameters json.RawMessage `json:"parameters"`
	} `json:"rules"`
}

type pullRequestParams struct {
	RequiredApprovingReviewCount   int      `json:"required_approving_review_count"`
	DismissStaleReviewsOnPush      bool     `json:"dismiss_stale_reviews_on_push"`
	RequireCodeOwnerReview         bool     `json:"require_code_owner_review"`
	RequireLastPushApproval        bool     `json:"require_last_push_approval"`
	RequiredReviewThreadResolution bool     `json:"required_review_thread_resolution"`
	AllowedMergeMethods            []string `json:"allowed_merge_methods"`
}

type statusCheckContext struct {
	Context string `json:"context"`
}

type statusChecksParams struct {
	StrictRequiredStatusChecksPolicy bool                 `json:"strict_required_status_checks_policy"`
	RequiredStatusChecks             []statusCheckContext `json:"required_status_checks"`
}

// ruleRequirement is a rule type the policy demands plus a validator for its
// parameters. validate returns the ways the parameters fall short of the
// policy; nil means compliant. A nil validate accepts any parameters.
type ruleRequirement struct {
	ruleType string
	validate func(params json.RawMessage) []string
}

const ruleTypeRequiredStatusChecks = "required_status_checks"

// validatePullRequestParams reports how pull_request rule parameters fall
// short of the policy; nil means compliant.
func validatePullRequestParams(rules policy.RulesetRules, raw json.RawMessage) []string {
	var p pullRequestParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return []string{"unreadable parameters"}
		}
	}
	var problems []string
	if p.RequiredApprovingReviewCount < rules.RequiredApprovals {
		problems = append(problems, fmt.Sprintf("required_approving_review_count is %d, want at least %d",
			p.RequiredApprovingReviewCount, rules.RequiredApprovals))
	}
	for _, b := range []struct {
		want bool
		got  bool
		name string
	}{
		{rules.DismissStaleReviews, p.DismissStaleReviewsOnPush, "dismiss_stale_reviews_on_push"},
		{rules.RequireCodeOwnerReview, p.RequireCodeOwnerReview, "require_code_owner_review"},
		{rules.RequireLastPushApproval, p.RequireLastPushApproval, "require_last_push_approval"},
		{rules.RequireThreadResolution, p.RequiredReviewThreadResolution, "required_review_thread_resolution"},
	} {
		if b.want && !b.got {
			problems = append(problems, b.name+" is disabled")
		}
	}
	problems = append(problems, validateMergeMethods(rules, p.AllowedMergeMethods)...)
	return problems
}

// validateMergeMethods reports merge methods the ruleset allows but the
// policy does not. Absent/empty in the ruleset means every method is allowed.
func validateMergeMethods(rules policy.RulesetRules, got []string) []string {
	if len(rules.AllowedMergeMethods) == 0 {
		return nil
	}
	if len(got) == 0 {
		got = []string{"merge", "squash", "rebase"}
	}
	var problems []string
	for _, m := range got {
		if !slices.Contains(rules.AllowedMergeMethods, policy.MergeType(m)) {
			problems = append(problems, fmt.Sprintf("merge method %q allowed but not permitted by policy", m))
		}
	}
	return problems
}

// validateStatusChecksParams reports how required_status_checks rule
// parameters fall short of the policy; nil means compliant.
func validateStatusChecksParams(rules policy.RulesetRules, raw json.RawMessage) []string {
	var p statusChecksParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return []string{"unreadable parameters"}
		}
	}
	var problems []string
	for _, want := range rules.RequiredStatusChecks {
		found := slices.ContainsFunc(p.RequiredStatusChecks, func(c statusCheckContext) bool {
			return c.Context == want
		})
		if !found {
			problems = append(problems, fmt.Sprintf("status check %q not required", want))
		}
	}
	if rules.StrictStatusChecks && !p.StrictRequiredStatusChecksPolicy {
		problems = append(problems, "strict_required_status_checks_policy is disabled")
	}
	return problems
}

func requiredRules(pol policy.Policy) []ruleRequirement {
	var out []ruleRequirement
	rules := pol.Checks.Rulesets.Rules
	if rules.BlockDeletion {
		out = append(out, ruleRequirement{ruleType: "deletion"})
	}
	if rules.BlockForcePush {
		out = append(out, ruleRequirement{ruleType: "non_fast_forward"})
	}
	if rules.RequireSignatures {
		out = append(out, ruleRequirement{ruleType: "required_signatures"})
	}
	if rules.RequireLinearHistory {
		out = append(out, ruleRequirement{ruleType: "required_linear_history"})
	}
	if rules.RequirePR {
		out = append(out, ruleRequirement{ruleType: "pull_request", validate: func(raw json.RawMessage) []string {
			return validatePullRequestParams(rules, raw)
		}})
	}
	if len(rules.RequiredStatusChecks) > 0 {
		out = append(
			out,
			ruleRequirement{ruleType: ruleTypeRequiredStatusChecks, validate: func(raw json.RawMessage) []string {
				return validateStatusChecksParams(rules, raw)
			}},
		)
	}
	return out
}

// coveredRules unions the rules across active rulesets covering the default
// branch, keeping each instance's parameters for validation.
func coveredRules(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	base string,
	summaries []rulesetSummary,
) (map[string][]json.RawMessage, error) {
	covered := map[string][]json.RawMessage{}
	for _, s := range summaries {
		if s.Enforcement != "active" || s.Target != "branch" {
			continue
		}
		var full ruleset
		if err := client.Get(ctx, fmt.Sprintf("%s/%d", base, s.ID), &full); err != nil {
			// Org-inherited rulesets appear in the repo list but their detail
			// endpoint 404s at the repo level; they can't be inspected here.
			if githubapi.StatusCode(err) == http.StatusNotFound {
				continue
			}
			return nil, err
		}
		include := full.Conditions.RefName.Include
		coversDefault := slices.Contains(include, "~DEFAULT_BRANCH") ||
			slices.Contains(include, "~ALL") ||
			slices.Contains(include, "refs/heads/"+repo.DefaultBranch)
		if !coversDefault {
			continue
		}
		for _, rule := range full.Rules {
			covered[rule.Type] = append(covered[rule.Type], rule.Parameters)
		}
	}
	return covered, nil
}

// closestProblems returns nil when any instance's parameters comply;
// otherwise the shortfalls of the closest instance.
func closestProblems(validate func(json.RawMessage) []string, instances []json.RawMessage) []string {
	var best []string
	for _, params := range instances {
		problems := validate(params)
		if len(problems) == 0 {
			return nil
		}
		if best == nil || len(problems) < len(best) {
			best = problems
		}
	}
	return best
}

func (r *Rulesets) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	pol policy.Policy,
) check.Result {
	base := fmt.Sprintf("repos/%s/%s/rulesets", repo.Owner, repo.Name)
	var summaries []rulesetSummary
	if err := client.Get(ctx, base+"?per_page=100", &summaries); err != nil {
		return check.Result{Error: err}
	}

	covered, err := coveredRules(ctx, client, repo, base, summaries)
	if err != nil {
		return check.Result{Error: err}
	}

	var missing []string
	var findings []check.Finding
	for _, want := range requiredRules(pol) {
		instances, ok := covered[want.ruleType]
		if !ok {
			missing = append(missing, want.ruleType)
			continue
		}
		if want.validate == nil {
			continue
		}
		if best := closestProblems(want.validate, instances); best != nil {
			findings = append(findings, check.Finding{
				Message: fmt.Sprintf("%s rule on default branch %q does not match policy: %s",
					want.ruleType, repo.DefaultBranch, strings.Join(best, "; ")),
				FixHint: "update the ruleset's rule parameters to match the policy",
			})
		}
	}
	if len(missing) > 0 {
		findings = append([]check.Finding{
			{
				Message: fmt.Sprintf(
					"default branch %q missing rules: %s",
					repo.DefaultBranch,
					strings.Join(missing, ", "),
				),
				FixHint: "create a ruleset protecting the default branch",
			},
		}, findings...)
	}
	if len(findings) == 0 {
		return check.Result{Status: check.Pass}
	}
	return check.Result{Status: check.Fail, Findings: findings}
}

// ruleEntry builds a ruleset rule payload; params may be nil for
// parameterless rules.
func ruleEntry(ruleType string, params map[string]any) map[string]any {
	entry := map[string]any{"type": ruleType}
	if params != nil {
		entry["parameters"] = params
	}
	return entry
}

func (r *Rulesets) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	rules := []map[string]any{}
	polRules := pol.Checks.Rulesets.Rules
	if polRules.BlockDeletion {
		rules = append(rules, ruleEntry("deletion", nil))
	}
	if polRules.BlockForcePush {
		rules = append(rules, ruleEntry("non_fast_forward", nil))
	}
	if polRules.RequireSignatures {
		rules = append(rules, ruleEntry("required_signatures", nil))
	}
	if polRules.RequireLinearHistory {
		rules = append(rules, ruleEntry("required_linear_history", nil))
	}
	if polRules.RequirePR {
		params := map[string]any{
			"required_approving_review_count":   polRules.RequiredApprovals,
			"dismiss_stale_reviews_on_push":     polRules.DismissStaleReviews,
			"require_code_owner_review":         polRules.RequireCodeOwnerReview,
			"require_last_push_approval":        polRules.RequireLastPushApproval,
			"required_review_thread_resolution": polRules.RequireThreadResolution,
		}
		if len(polRules.AllowedMergeMethods) > 0 {
			params["allowed_merge_methods"] = polRules.AllowedMergeMethods
		}
		rules = append(rules, ruleEntry("pull_request", params))
	}
	if len(polRules.RequiredStatusChecks) > 0 {
		contexts := make([]map[string]any, 0, len(polRules.RequiredStatusChecks))
		for _, c := range polRules.RequiredStatusChecks {
			contexts = append(contexts, map[string]any{"context": c})
		}
		rules = append(rules, ruleEntry(ruleTypeRequiredStatusChecks, map[string]any{
			ruleTypeRequiredStatusChecks:           contexts,
			"strict_required_status_checks_policy": polRules.StrictStatusChecks,
		}))
	}
	payload := map[string]any{
		"name":        "repocheck: protect default branch",
		"target":      "branch",
		"enforcement": "active",
		"conditions": map[string]any{
			"ref_name": map[string]any{
				"include": []string{"~DEFAULT_BRANCH"},
				"exclude": []string{},
			},
		},
		"rules": rules,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s/rulesets", repo.Owner, repo.Name)
	return client.Post(ctx, path, bytes.NewReader(body), nil)
}
