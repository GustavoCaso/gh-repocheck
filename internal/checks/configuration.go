package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type Configuration struct{}

var _ check.Fixable = (*Configuration)(nil)

func (c *Configuration) ID() string          { return "configuration" }
func (c *Configuration) Description() string { return "Repository settings match the policy" }

func (c *Configuration) Enabled(pol policy.Policy) bool { return pol.Checks.Configuration.Enabled }

type configuration struct {
	HasIssues                bool `json:"has_issues"`
	HasProjects              bool `json:"has_projects"`
	HasWiki                  bool `json:"has_wiki"`
	AllowSquashMerge         bool `json:"allow_squash_merge"`
	AllowMergeCommit         bool `json:"allow_merge_commit"`
	AllowRebaseMerge         bool `json:"allow_rebase_merge"`
	AllowAutoMerge           bool `json:"allow_auto_merge"`
	DeleteBranchOnMerge      bool `json:"delete_branch_on_merge"`
	AllowForking             bool `json:"allow_forking"`
	WebCommitSignoffRequired bool `json:"web_commit_signoff_required"`
}

func (c *configuration) settings() []struct {
	name  string
	value bool
} {
	return []struct {
		name  string
		value bool
	}{
		{"has_issues", c.HasIssues},
		{"has_projects", c.HasProjects},
		{"has_wiki", c.HasWiki},
		{"allow_squash_merge", c.AllowSquashMerge},
		{"allow_merge_commit", c.AllowMergeCommit},
		{"allow_rebase_merge", c.AllowRebaseMerge},
		{"allow_auto_merge", c.AllowAutoMerge},
		{"delete_branch_on_merge", c.DeleteBranchOnMerge},
		{"allow_forking", c.AllowForking},
		{"web_commit_signoff_required", c.WebCommitSignoffRequired},
	}
}

func desired(pol policy.Policy) configuration {
	p := pol.Checks.Configuration
	return configuration{
		HasIssues:                p.HasIssues,
		HasProjects:              p.HasProjects,
		HasWiki:                  p.HasWiki,
		AllowSquashMerge:         p.AllowSquashMerge,
		AllowMergeCommit:         p.AllowMergeCommit,
		AllowRebaseMerge:         p.AllowRebaseMerge,
		AllowAutoMerge:           p.AllowAutoMerge,
		DeleteBranchOnMerge:      p.DeleteBranchOnMerge,
		AllowForking:             p.AllowForking,
		WebCommitSignoffRequired: p.WebCommitSignoffRequired,
	}
}

func (c *Configuration) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	pol policy.Policy,
) check.Result {
	var cfg configuration
	path := fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name)
	if err := client.Get(ctx, path, &cfg); err != nil {
		return check.Result{Error: err}
	}

	want := desired(pol)
	got := cfg.settings()
	var findings []check.Finding
	for i, w := range want.settings() {
		if got[i].value != w.value {
			findings = append(findings, check.Finding{
				Message: fmt.Sprintf("`%s` is %t, policy wants %t", w.name, got[i].value, w.value),
				FixHint: fmt.Sprintf("set `%s` to %t", w.name, w.value),
			})
		}
	}
	if len(findings) > 0 {
		return check.Result{Status: check.Fail, Findings: findings}
	}
	return check.Result{Status: check.Pass}
}

func (c *Configuration) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	body, err := json.Marshal(desired(pol))
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name)
	return client.Patch(ctx, path, bytes.NewReader(body), nil)
}
