package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type stubCheck struct {
	id     string
	result check.Result
	err    error
}

func (s *stubCheck) ID() string          { return s.id }
func (s *stubCheck) Description() string { return s.id }
func (s *stubCheck) Run(context.Context, githubapi.Client, check.Repo, policy.Policy) (check.Result, error) {
	return s.result, s.err
}

func TestRunRepoCollectsResultsInCheckOrder(t *testing.T) {
	checks := []check.Check{
		&stubCheck{id: "a", result: check.Result{Status: check.Pass}},
		&stubCheck{id: "b", result: check.Result{Status: check.Fail, Findings: []check.Finding{{Message: "bad"}}}},
	}
	repo := check.Repo{Owner: "o", Name: "r"}
	results := RunRepo(context.Background(), nil, checks, repo, policy.Defaults())
	if len(results) != 2 || results[0].Check.ID() != "a" || results[1].Result.Status != check.Fail {
		t.Errorf("results = %+v", results)
	}
}

func TestRunRepoCheckErrorBecomesErrorResult(t *testing.T) {
	checks := []check.Check{&stubCheck{id: "a", err: errors.New("boom")}}
	results := RunRepo(context.Background(), nil, checks, check.Repo{Owner: "o", Name: "r"}, policy.Defaults())
	if results[0].Err == nil {
		t.Error("check error should be captured, not dropped")
	}
}

func TestRunRepoSkipsDisabledChecks(t *testing.T) {
	pol := policy.Defaults()
	pol.Checks.CodeQL.Enabled = false
	checks := []check.Check{&stubCheck{id: "codeql", result: check.Result{Status: check.Fail}}}
	results := RunRepo(context.Background(), nil, checks, check.Repo{Owner: "o", Name: "r"}, pol)
	if results[0].Result.Status != check.Skip {
		t.Errorf("disabled check should report Skip, got %v", results[0].Result.Status)
	}
}

func TestRunReposPreservesRepoOrder(t *testing.T) {
	checks := []check.Check{&stubCheck{id: "a", result: check.Result{Status: check.Pass}}}
	repos := []check.Repo{
		{Owner: "o", Name: "r1"},
		{Owner: "o", Name: "r2"},
		{Owner: "o", Name: "r3"},
		{Owner: "o", Name: "r4"},
		{Owner: "o", Name: "r5"},
		{Owner: "o", Name: "r6"},
		{Owner: "o", Name: "r7"},
	}
	out := RunRepos(context.Background(), nil, checks, repos, policy.Defaults())
	if len(out) != len(repos) {
		t.Fatalf("got %d repo results, want %d", len(out), len(repos))
	}
	for i, results := range out {
		if len(results) != 1 {
			t.Fatalf("repo %d: got %d results, want 1", i, len(results))
		}
		if results[0].Repo.Name != repos[i].Name {
			t.Errorf("out[%d] is for %s, want %s", i, results[0].Repo.Name, repos[i].Name)
		}
		if results[0].Result.Status != check.Pass {
			t.Errorf("repo %d: status = %v", i, results[0].Result.Status)
		}
	}
}
