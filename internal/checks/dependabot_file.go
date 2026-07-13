package checks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type DependabotFile struct{}

func (d *DependabotFile) ID() string { return "dependabot-file" }
func (d *DependabotFile) Description() string {
	return "Dependabot file is present in the repository"
}

func (d *DependabotFile) Run(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) (check.Result, error) {
	base := fmt.Sprintf("repos/%s/%s/contents/.github/dependabot.yml", repo.Owner, repo.Name)
	var findings []check.Finding
	failed := false

	// dependabot.yml presence (version updates): 404 means missing.
	if err := client.Get(ctx, base, nil); err != nil {
		if githubapi.StatusCode(err) != http.StatusNotFound {
			return check.Result{}, err
		}
		f := check.Finding{
			Message: "no .github/dependabot.yml (version updates not configured)",
		}
		findings = append(findings, f)
		failed = true
	}

	switch {
	case failed:
		return check.Result{Status: check.Fail, Findings: findings}, nil
	default:
		return check.Result{Status: check.Pass}, nil
	}
}
