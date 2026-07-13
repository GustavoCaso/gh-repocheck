package checks

import (
	"context"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

var testRepo = check.Repo{Owner: "o", Name: "r", DefaultBranch: "main"}

func run(t *testing.T, c check.Check, stub *githubapi.Stub) check.Result {
	t.Helper()
	return runWithPolicy(t, c, stub, policy.Defaults())
}

func runWithPolicy(t *testing.T, c check.Check, stub *githubapi.Stub, pol policy.Policy) check.Result {
	t.Helper()
	res, err := c.Run(context.Background(), stub, testRepo, pol)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
