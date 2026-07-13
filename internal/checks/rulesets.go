package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
			if len(rules.AllowedMergeMethods) > 0 {
				// Absent/empty in the ruleset means every method is allowed.
				got := p.AllowedMergeMethods
				if len(got) == 0 {
					got = []string{"merge", "squash", "rebase"}
				}
				for _, m := range got {
					if !slices.Contains(rules.AllowedMergeMethods, policy.MergeType(m)) {
						problems = append(problems, fmt.Sprintf("merge method %q allowed but not permitted by policy", m))
					}
				}
			}
			return problems
		}})
	}
	if len(rules.RequiredStatusChecks) > 0 {
		out = append(out, ruleRequirement{ruleType: "required_status_checks", validate: func(raw json.RawMessage) []string {
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
		}})
	}
	return out
}

func (r *Rulesets) Run(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) (check.Result, error) {
	base := fmt.Sprintf("repos/%s/%s/rulesets", repo.Owner, repo.Name)
	var summaries []rulesetSummary
	if err := client.Get(ctx, base+"?per_page=100", &summaries); err != nil {
		return check.Result{}, err
	}

	// Union of rules across active rulesets covering the default branch,
	// keeping each instance's parameters for validation.
	covered := map[string][]json.RawMessage{}
	for _, s := range summaries {
		if s.Enforcement != "active" || s.Target != "branch" {
			continue
		}
		var full ruleset
		if err := client.Get(ctx, fmt.Sprintf("%s/%d", base, s.ID), &full); err != nil {
			// Org-inherited rulesets appear in the repo list but their detail
			// endpoint 404s at the repo level; they can't be inspected here.
			if githubapi.StatusCode(err) == 404 {
				continue
			}
			return check.Result{}, err
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
		// Satisfied if any covering ruleset's instance complies; otherwise
		// report the closest instance's shortfalls.
		var best []string
		for _, params := range instances {
			problems := want.validate(params)
			if len(problems) == 0 {
				best = nil
				break
			}
			if best == nil || len(problems) < len(best) {
				best = problems
			}
		}
		if best != nil {
			findings = append(findings, check.Finding{
				Message: fmt.Sprintf("%s rule on default branch %q does not match policy: %s",
					want.ruleType, repo.DefaultBranch, strings.Join(best, "; ")),
				FixHint: "update the ruleset's rule parameters to match the policy",
			})
		}
	}
	if len(missing) > 0 {
		findings = append([]check.Finding{{
			Message: fmt.Sprintf("default branch %q missing rules: %s", repo.DefaultBranch, strings.Join(missing, ", ")),
			FixHint: "create a ruleset protecting the default branch",
		}}, findings...)
	}
	if len(findings) == 0 {
		return check.Result{Status: check.Pass}, nil
	}
	return check.Result{Status: check.Fail, Findings: findings}, nil
}

func (r *Rulesets) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	rules := []map[string]any{}
	polRules := pol.Checks.Rulesets.Rules
	if polRules.BlockDeletion {
		rules = append(rules, map[string]any{"type": "deletion"})
	}
	if polRules.BlockForcePush {
		rules = append(rules, map[string]any{"type": "non_fast_forward"})
	}
	if polRules.RequireSignatures {
		rules = append(rules, map[string]any{"type": "required_signatures"})
	}
	if polRules.RequireLinearHistory {
		rules = append(rules, map[string]any{"type": "required_linear_history"})
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
		rules = append(rules, map[string]any{"type": "pull_request", "parameters": params})
	}
	if len(polRules.RequiredStatusChecks) > 0 {
		contexts := make([]map[string]any, 0, len(polRules.RequiredStatusChecks))
		for _, c := range polRules.RequiredStatusChecks {
			contexts = append(contexts, map[string]any{"context": c})
		}
		rules = append(rules, map[string]any{
			"type": "required_status_checks",
			"parameters": map[string]any{
				"required_status_checks":               contexts,
				"strict_required_status_checks_policy": polRules.StrictStatusChecks,
			},
		})
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
