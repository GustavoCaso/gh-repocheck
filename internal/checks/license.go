package checks

import (
	"context"
	"fmt"
	"slices"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// License detects but never fixes: choosing a license is a human decision.
type License struct{}

func (l *License) ID() string          { return "license" }
func (l *License) Description() string { return "Repository has a license" }

func (l *License) Enabled(pol policy.Policy) bool { return pol.Checks.License.Enabled }

func (l *License) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	pol policy.Policy,
) check.Result {
	var resp struct {
		License *struct {
			SPDXID string `json:"spdx_id"`
		} `json:"license"`
	}
	if err := client.Get(ctx, fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name), &resp); err != nil {
		return check.Result{Error: err}
	}
	if resp.License == nil {
		return check.Result{Status: check.Fail, Findings: []check.Finding{{
			Message: "no license file — pick one at https://choosealicense.com and add a LICENSE file",
		}}}
	}
	allowed := pol.Checks.License.Allowed
	if len(allowed) > 0 && !slices.Contains(allowed, resp.License.SPDXID) {
		return check.Result{Status: check.Fail, Findings: []check.Finding{{
			Message: fmt.Sprintf("license %s not in allowed list %v", resp.License.SPDXID, allowed),
		}}}
	}
	return check.Result{Status: check.Pass}
}
