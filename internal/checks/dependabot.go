package checks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type Dependabot struct{}

var _ check.Fixable = (*Dependabot)(nil)

func (d *Dependabot) ID() string { return "dependabot" }
func (d *Dependabot) Description() string {
	return "Dependabot alerts and automated security fixes are enabled"
}

func (d *Dependabot) Enabled(pol policy.Policy) bool { return pol.Checks.Dependabot.Enabled }

func (d *Dependabot) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	_ policy.Policy,
) check.Result {
	base := fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name)
	var findings []check.Finding
	failed := false

	// Vulnerability alerts: 204 (nil error) enabled, 404 disabled.
	if err := client.Get(ctx, base+"/vulnerability-alerts", nil); err != nil {
		if githubapi.StatusCode(err) != http.StatusNotFound {
			return check.Result{Error: err}
		}
		failed = true
		findings = append(findings, check.Finding{
			Message: "Dependabot vulnerability alerts are disabled",
			FixHint: "enable vulnerability alerts",
		})
	}

	var secFixes struct {
		Enabled bool `json:"enabled"`
	}
	if err := client.Get(ctx, base+"/automated-security-fixes", &secFixes); err != nil {
		if githubapi.StatusCode(err) != http.StatusNotFound {
			return check.Result{Error: err}
		}
	}
	if !secFixes.Enabled {
		failed = true
		findings = append(findings, check.Finding{
			Message: "Dependabot automated security fixes are disabled",
			FixHint: "enable automated security fixes",
		})
	}

	switch {
	case failed:
		return check.Result{Status: check.Fail, Findings: findings}
	default:
		return check.Result{Status: check.Pass}
	}
}

func (d *Dependabot) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, _ policy.Policy) error {
	base := fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name)
	if err := client.Put(ctx, base+"/vulnerability-alerts", nil, nil); err != nil {
		return fmt.Errorf("enabling vulnerability alerts: %w", err)
	}
	if err := client.Put(ctx, base+"/automated-security-fixes", nil, nil); err != nil {
		return fmt.Errorf("enabling automated security fixes: %w", err)
	}
	return nil
}
