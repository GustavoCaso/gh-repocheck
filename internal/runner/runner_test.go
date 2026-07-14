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
	id       string
	disabled bool
	result   check.Result
	err      error
}

func (s *stubCheck) ID() string                 { return s.id }
func (s *stubCheck) Description() string        { return s.id }
func (s *stubCheck) Enabled(policy.Policy) bool { return !s.disabled }
func (s *stubCheck) Run(context.Context, githubapi.Client, check.Repo, policy.Policy) check.Result {
	r := s.result
	r.Error = s.err
	return r
}

func TestRunRepoCollectsResultsInCheckOrder(t *testing.T) {
	checks := []check.Check{
		&stubCheck{id: "a", result: check.Result{Status: check.Pass}},
		&stubCheck{id: "b", result: check.Result{Status: check.Fail, Findings: []check.Finding{{Message: "bad"}}}},
	}
	repo := check.Repo{Owner: "o", Name: "r"}
	results := RunRepo(context.Background(), nil, checks, repo, policy.Defaults())
	if len(results) != 2 || results[0].Check.ID() != "a" || results[1].Status != check.Fail {
		t.Errorf("results = %+v", results)
	}
}

func TestRunRepoStampsCheckAndRepo(t *testing.T) {
	checks := []check.Check{&stubCheck{id: "a", result: check.Result{Status: check.Pass}}}
	repo := check.Repo{Owner: "o", Name: "r"}
	results := RunRepo(context.Background(), nil, checks, repo, policy.Defaults())
	if results[0].Check == nil || results[0].Check.ID() != "a" || results[0].Repo.Name != "r" {
		t.Errorf("runner must stamp Check and Repo, got %+v", results[0])
	}
}

func TestRunRepoSkipsDisabledChecks(t *testing.T) {
	checks := []check.Check{&stubCheck{id: "a", disabled: true, result: check.Result{Status: check.Fail}}}
	results := RunRepo(context.Background(), nil, checks, check.Repo{Owner: "o", Name: "r"}, policy.Defaults())
	r := results[0]
	if r.Status != check.Skip {
		t.Errorf("disabled check should report Skip, got %v", r.Status)
	}
	if r.Check == nil || r.Check.ID() != "a" || r.Repo.Name != "r" {
		t.Errorf("skip result must carry Check and Repo, got %+v", r)
	}
}

func TestRunRepoCheckErrorBecomesErrorResult(t *testing.T) {
	checks := []check.Check{&stubCheck{id: "a", err: errors.New("boom")}}
	results := RunRepo(context.Background(), nil, checks, check.Repo{Owner: "o", Name: "r"}, policy.Defaults())
	if results[0].Error == nil {
		t.Error("check error should be captured, not dropped")
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
		if results[0].Status != check.Pass {
			t.Errorf("repo %d: status = %v", i, results[0].Status)
		}
	}
}
