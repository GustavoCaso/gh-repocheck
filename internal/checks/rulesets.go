package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type Rulesets struct{}

var _ check.Fixable = (*Rulesets)(nil)

func (r *Rulesets) ID() string          { return "rulesets" }
func (r *Rulesets) Description() string { return "Named rulesets protect the configured branches" }

func (r *Rulesets) Enabled(pol policy.Policy) bool { return pol.Checks.Rulesets.Enabled }

// GitHub actor_id values for actor_type RepositoryRole.
const (
	maintainRoleID = 2
	writeRoleID    = 4
	adminRoleID    = 5
)

//nolint:gochecknoglobals // acceptable
var roleNames = map[int]string{
	maintainRoleID: "maintain",
	writeRoleID:    "write",
	adminRoleID:    "admin",
}

const repositoryRoleActorType = "RepositoryRole"

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

func (r *Rulesets) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	pol policy.Policy,
) check.Result {
	rulesets, err := githubapi.FetchRulesets(ctx, client, repo.Owner, repo.Name,
		policyRulesetNames(pol)...)
	if err != nil {
		return check.Result{Error: err}
	}
	byName := map[string]githubapi.Ruleset{}
	for _, rs := range rulesets {
		byName[rs.Name] = rs
	}

	var findings []check.Finding
	for _, branch := range sortedBranches(pol.Checks.Rulesets.Rules) {
		for _, want := range pol.Checks.Rulesets.Rules[branch] {
			rs, ok := byName[want.Name]
			if !ok {
				findings = append(findings, check.Finding{
					Message: fmt.Sprintf("ruleset %q covering branch %q not found", want.Name, branch),
					FixHint: fmt.Sprintf("create ruleset %q from the policy", want.Name),
				})
				continue
			}
			// Org-inherited rulesets can't be inspected at the repo level;
			// the name exists, so don't fail on what can't be verified.
			if rs.Uninspectable {
				continue
			}
			if problems := validateRuleset(rs, branch, repo, want); len(problems) > 0 {
				findings = append(findings, check.Finding{
					Message: fmt.Sprintf("ruleset %q for branch %q does not match policy: %s",
						want.Name, branch, strings.Join(problems, "; ")),
					FixHint: fmt.Sprintf("update ruleset %q to match the policy", want.Name),
				})
			}
		}
	}
	if len(findings) == 0 {
		return check.Result{Status: check.Pass}
	}
	return check.Result{Status: check.Fail, Findings: findings}
}

//nolint:gocognit // acceptable
func (r *Rulesets) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	rulesets, err := githubapi.FetchRulesets(ctx, client, repo.Owner, repo.Name,
		policyRulesetNames(pol)...)
	if err != nil {
		return err
	}
	uninspectable := map[string]bool{}
	idByName := map[string]int{}
	for _, rs := range rulesets {
		if rs.Uninspectable {
			uninspectable[rs.Name] = true
			continue
		}
		idByName[rs.Name] = rs.ID
	}

	for _, branch := range sortedBranches(pol.Checks.Rulesets.Rules) {
		for _, want := range pol.Checks.Rulesets.Rules[branch] {
			// Org-inherited rulesets can't be updated at the repo level.
			if uninspectable[want.Name] {
				continue
			}
			body, bodyErr := json.Marshal(desiredRuleset(branch, want))
			if bodyErr != nil {
				return bodyErr
			}
			if id, ok := idByName[want.Name]; ok {
				path := fmt.Sprintf("repos/%s/%s/rulesets/%d", repo.Owner, repo.Name, id)
				if putErr := client.Put(ctx, path, bytes.NewReader(body), nil); putErr != nil {
					return putErr
				}
				continue
			}
			path := fmt.Sprintf("repos/%s/%s/rulesets", repo.Owner, repo.Name)
			if postErr := client.Post(ctx, path, bytes.NewReader(body), nil); postErr != nil {
				return postErr
			}
		}
	}
	return nil
}

// policyRulesetNames collects the ruleset names the policy declares, so
// only those rulesets' details are fetched.
func policyRulesetNames(pol policy.Policy) []string {
	seen := map[string]bool{}
	var names []string
	for _, wants := range pol.Checks.Rulesets.Rules {
		for _, want := range wants {
			if !seen[want.Name] {
				seen[want.Name] = true
				names = append(names, want.Name)
			}
		}
	}
	return names
}

func sortedBranches(rules map[string][]policy.RulesetRules) []string {
	branches := make([]string, 0, len(rules))
	for b := range rules {
		branches = append(branches, b)
	}
	sort.Strings(branches)
	return branches
}

// validateRuleset reports how a ruleset falls short of the policy for one
// branch; nil means compliant.
func validateRuleset(
	rs githubapi.Ruleset,
	branch string,
	repo check.Repo,
	want policy.RulesetRules,
) []string {
	var problems []string
	if rs.Enforcement != "active" {
		problems = append(problems, fmt.Sprintf("enforcement is %q, want active", rs.Enforcement))
	}
	if rs.Target != "branch" {
		problems = append(problems, fmt.Sprintf("target is %q, want branch", rs.Target))
	}
	include := rs.Conditions.RefName.Include
	covers := slices.Contains(include, "refs/heads/"+branch) ||
		slices.Contains(include, "~ALL") ||
		(branch == repo.DefaultBranch && slices.Contains(include, "~DEFAULT_BRANCH"))
	if !covers {
		problems = append(problems, fmt.Sprintf("conditions do not include %q", "refs/heads/"+branch))
	}
	byType := map[string]json.RawMessage{}
	for _, rule := range rs.Rules {
		byType[rule.Type] = rule.Parameters
	}
	for _, req := range requiredRules(want) {
		raw, ok := byType[req.ruleType]
		if !ok {
			problems = append(problems, "missing rule: "+req.ruleType)
			continue
		}
		if req.validate != nil {
			problems = append(problems, req.validate(raw)...)
		}
	}
	problems = append(problems, validateBypassActors(want, rs.BypassActors)...)
	return problems
}

// validateBypassActors compares the ruleset's repository-role bypass actors
// against the policy. Actors of other types (teams, apps) are not modeled by
// the policy and are ignored.
func validateBypassActors(rules policy.RulesetRules, actual []githubapi.BypassActor) []string {
	var problems []string
	desired := desiredBypassActors(rules)
	for _, d := range desired {
		i := slices.IndexFunc(actual, func(a githubapi.BypassActor) bool {
			return a.ActorType == repositoryRoleActorType && a.ActorID == d.ActorID
		})
		if i < 0 {
			problems = append(problems, fmt.Sprintf("bypass for %s role missing", roleNames[d.ActorID]))
			continue
		}
		if actual[i].BypassMode != d.BypassMode {
			problems = append(problems, fmt.Sprintf("bypass mode for %s role is %q, want %q",
				roleNames[d.ActorID], actual[i].BypassMode, d.BypassMode))
		}
	}
	for _, a := range actual {
		if a.ActorType != repositoryRoleActorType {
			continue
		}
		allowed := slices.ContainsFunc(desired, func(d githubapi.BypassActor) bool {
			return d.ActorID == a.ActorID
		})
		if !allowed {
			name := roleNames[a.ActorID]
			if name == "" {
				name = fmt.Sprintf("id %d", a.ActorID)
			}
			problems = append(problems, fmt.Sprintf("bypass for %s role not permitted by policy", name))
		}
	}
	return problems
}

func desiredBypassActors(rules policy.RulesetRules) []githubapi.BypassActor {
	mode := func(m policy.BypassMode) string {
		if m == "" {
			return string(policy.AlwaysMode)
		}
		return string(m)
	}
	var actors []githubapi.BypassActor
	if rules.BypassByAdminRole {
		actors = append(actors, githubapi.BypassActor{
			ActorID: adminRoleID, ActorType: repositoryRoleActorType, BypassMode: mode(rules.BypassModeAdmin),
		})
	}
	if rules.BypassByMaintainerRole {
		actors = append(actors, githubapi.BypassActor{
			ActorID: maintainRoleID, ActorType: repositoryRoleActorType, BypassMode: mode(rules.BypassModeMaintainer),
		})
	}
	if rules.BypassByWriterRole {
		actors = append(actors, githubapi.BypassActor{
			ActorID: writeRoleID, ActorType: repositoryRoleActorType, BypassMode: mode(rules.BypassModeWriter),
		})
	}
	return actors
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

// desiredRuleset builds the full ruleset payload the policy wants for one
// branch; the same payload serves POST (create) and PUT (replace).
func desiredRuleset(branch string, want policy.RulesetRules) map[string]any {
	rules := []map[string]any{}
	if want.BlockDeletion {
		rules = append(rules, ruleEntry("deletion", nil))
	}
	if want.BlockForcePush {
		rules = append(rules, ruleEntry("non_fast_forward", nil))
	}
	if want.RequireSignatures {
		rules = append(rules, ruleEntry("required_signatures", nil))
	}
	if want.RequireLinearHistory {
		rules = append(rules, ruleEntry("required_linear_history", nil))
	}
	if want.RequirePR {
		params := map[string]any{
			"required_approving_review_count":   want.RequiredApprovals,
			"dismiss_stale_reviews_on_push":     want.DismissStaleReviews,
			"require_code_owner_review":         want.RequireCodeOwnerReview,
			"require_last_push_approval":        want.RequireLastPushApproval,
			"required_review_thread_resolution": want.RequireThreadResolution,
		}
		if len(want.AllowedMergeMethods) > 0 {
			params["allowed_merge_methods"] = want.AllowedMergeMethods
		}
		rules = append(rules, ruleEntry("pull_request", params))
	}
	if len(want.RequiredStatusChecks) > 0 {
		contexts := make([]map[string]any, 0, len(want.RequiredStatusChecks))
		for _, c := range want.RequiredStatusChecks {
			contexts = append(contexts, map[string]any{"context": c})
		}
		rules = append(rules, ruleEntry(ruleTypeRequiredStatusChecks, map[string]any{
			ruleTypeRequiredStatusChecks:           contexts,
			"strict_required_status_checks_policy": want.StrictStatusChecks,
		}))
	}
	payload := map[string]any{
		"name":        want.Name,
		"target":      "branch",
		"enforcement": "active",
		"conditions": map[string]any{
			"ref_name": map[string]any{
				"include": []string{"refs/heads/" + branch},
				"exclude": []string{},
			},
		},
		"rules": rules,
	}
	if actors := desiredBypassActors(want); len(actors) > 0 {
		payload["bypass_actors"] = actors
	}
	return payload
}

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

func requiredRules(rules policy.RulesetRules) []ruleRequirement {
	var out []ruleRequirement
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
