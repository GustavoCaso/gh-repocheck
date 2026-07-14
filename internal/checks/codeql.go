package checks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type CodeQL struct{}

var _ check.Fixable = (*CodeQL)(nil)

func (c *CodeQL) ID() string          { return "codeql" }
func (c *CodeQL) Description() string { return "CodeQL default setup is enabled" }

func (c *CodeQL) Enabled(pol policy.Policy) bool { return pol.Checks.CodeQL.Enabled }

type setup struct {
	State     string   `json:"state"`
	Languages []string `json:"languages"`
}

func (c *CodeQL) Run(
	ctx context.Context,
	client githubapi.Client,
	repo check.Repo,
	_ policy.Policy,
) check.Result {
	var cfg setup
	path := fmt.Sprintf("repos/%s/%s/code-scanning/default-setup", repo.Owner, repo.Name)
	if err := client.Get(ctx, path, &cfg); err != nil {
		switch githubapi.StatusCode(err) {
		case http.StatusForbidden, http.StatusNotFound:
			return check.Result{Status: check.Skip, Findings: []check.Finding{{
				Message: "code scanning unavailable (requires public repo or GitHub Advanced Security)",
			}}}
		}
		return check.Result{Error: err}
	}
	if cfg.State == "configured" {
		return check.Result{Status: check.Pass}
	}
	if len(cfg.Languages) == 0 {
		return check.Result{Status: check.Skip, Findings: []check.Finding{{
			Message: "no CodeQL-supported languages detected",
		}}}
	}
	return check.Result{Status: check.Fail, Findings: []check.Finding{{
		Message: "CodeQL default setup is not enabled",
		FixHint: "enable CodeQL default setup",
	}}}
}

func (c *CodeQL) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, _ policy.Policy) error {
	path := fmt.Sprintf("repos/%s/%s/code-scanning/default-setup", repo.Owner, repo.Name)
	body := bytes.NewReader([]byte(`{"state":"configured"}`))
	return client.Patch(ctx, path, body, nil)
}
