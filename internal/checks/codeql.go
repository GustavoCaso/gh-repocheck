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

type setup struct {
	State     string   `json:"state"`
	Languages []string `json:"languages"`
}

func (c *CodeQL) Run(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) (check.Result, error) {
	var setup setup
	path := fmt.Sprintf("repos/%s/%s/code-scanning/default-setup", repo.Owner, repo.Name)
	if err := client.Get(ctx, path, &setup); err != nil {
		switch githubapi.StatusCode(err) {
		case http.StatusForbidden, http.StatusNotFound:
			return check.Result{Status: check.Skip, Findings: []check.Finding{{
				Message: "code scanning unavailable (requires public repo or GitHub Advanced Security)",
			}}}, nil
		}
		return check.Result{}, err
	}
	if setup.State == "configured" {
		return check.Result{Status: check.Pass}, nil
	}
	if len(setup.Languages) == 0 {
		return check.Result{Status: check.Skip, Findings: []check.Finding{{
			Message: "no CodeQL-supported languages detected",
		}}}, nil
	}
	return check.Result{Status: check.Fail, Findings: []check.Finding{{
		Message: "CodeQL default setup is not enabled",
		FixHint: "enable CodeQL default setup",
	}}}, nil
}

func (c *CodeQL) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	path := fmt.Sprintf("repos/%s/%s/code-scanning/default-setup", repo.Owner, repo.Name)
	body := bytes.NewReader([]byte(`{"state":"configured"}`))
	return client.Patch(ctx, path, body, nil)
}
