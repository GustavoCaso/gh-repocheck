// Package runner executes checks against repos.
package runner

import (
	"context"
	"sync"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// RunRepo runs the checks against one repo concurrently; results keep check order.
func RunRepo(
	ctx context.Context,
	client githubapi.Client,
	checks []check.Check,
	repo check.Repo,
	pol policy.Policy,
) []check.Result {
	results := make([]check.Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		if !c.Enabled(pol) {
			results[i] = check.Result{Check: c, Repo: repo, Status: check.Skip,
				Findings: []check.Finding{{Message: "disabled by policy"}}}
			continue
		}
		wg.Add(1)
		go func(i int, c check.Check) {
			defer wg.Done()
			r := c.Run(ctx, client, repo, pol)
			r.Check, r.Repo = c, repo
			results[i] = r
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
) [][]check.Result {
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	out := make([][]check.Result, len(repos))
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
