package checks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// Codeowners detects but never fixes: assigning code owners is a human decision.
type Codeowners struct{}

func (c *Codeowners) ID() string          { return "codeowners" }
func (c *Codeowners) Description() string { return "CODEOWNERS file is present and valid" }

func (c *Codeowners) Enabled(pol policy.Policy) bool { return pol.Checks.Codeowners.Enabled }

func (c *Codeowners) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	_ policy.Policy,
) check.Result {
	var resp struct {
		Errors []struct {
			Line    int    `json:"line"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	path := fmt.Sprintf("repos/%s/%s/codeowners/errors", repo.Owner, repo.Name)
	if err := client.Get(ctx, path, &resp); err != nil {
		if githubapi.StatusCode(err) != http.StatusNotFound {
			return check.Result{Error: err}
		}
		return check.Result{Status: check.Fail, Findings: []check.Finding{{
			Message: "no CODEOWNERS file (add one at .github/CODEOWNERS, CODEOWNERS, or docs/CODEOWNERS)",
		}}}
	}
	if len(resp.Errors) == 0 {
		return check.Result{Status: check.Pass}
	}
	findings := make([]check.Finding, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		findings = append(findings, check.Finding{
			Message: fmt.Sprintf("CODEOWNERS line %d: %s", e.Line, e.Message),
		})
	}
	return check.Result{Status: check.Fail, Findings: findings}
}
