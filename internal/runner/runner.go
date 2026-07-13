// Package runner executes checks against repos.
package runner

import (
	"context"
	"sync"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type CheckResult struct {
	Repo   check.Repo
	Check  check.Check
	Result check.Result
	Err    error
}

// Enabled reports whether the policy enables the given check id.
func Enabled(pol policy.Policy, id string) bool {
	switch id {
	case "secret-scanning":
		return pol.Checks.SecretScanning.Enabled
	case "codeql":
		return pol.Checks.CodeQL.Enabled
	case "dependabot":
		return pol.Checks.Dependabot.Enabled
	case "license":
		return pol.Checks.License.Enabled
	case "rulesets":
		return pol.Checks.Rulesets.Enabled
	}
	return true // unknown (third-party) checks default enabled
}

// RunRepo runs the checks against one repo concurrently; results keep check order.
func RunRepo(
	ctx context.Context,
	client githubapi.Client,
	checks []check.Check,
	repo check.Repo,
	pol policy.Policy,
) []CheckResult {
	results := make([]CheckResult, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		if !Enabled(pol, c.ID()) {
			results[i] = CheckResult{Repo: repo, Check: c, Result: check.Result{
				Status:   check.Skip,
				Findings: []check.Finding{{Message: "disabled by policy"}},
			}}
			continue
		}
		wg.Add(1)
		go func(i int, c check.Check) {
			defer wg.Done()
			res, err := c.Run(ctx, client, repo, pol)
			results[i] = CheckResult{Repo: repo, Check: c, Result: res, Err: err}
		}(i, c)
	}
	wg.Wait()
	return results
}

// RunRepos sweeps repos with bounded concurrency.
func RunRepos(
	ctx context.Context,
	client githubapi.Client,
	checks []check.Check,
	repos []check.Repo,
	pol policy.Policy,
) [][]CheckResult {
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	out := make([][]CheckResult, len(repos))
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func(i int, repo check.Repo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = RunRepo(ctx, client, checks, repo, pol)
		}(i, repo)
	}
	wg.Wait()
	return out
}
