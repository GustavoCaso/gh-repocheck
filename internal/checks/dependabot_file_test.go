package checks

import (
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
)

// GET contents/.github/dependabot.yml: 200 = exists.

func dependabotFileStub(configStatus int) *githubapi.Stub {
	return &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/contents/.github/dependabot.yml": {Status: configStatus},
	}}
}

func TestDependabotFilePass(t *testing.T) {
	stub := dependabotFileStub(200)
	if res := run(t, &DependabotFile{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestDependabotFailWhenNoFile(t *testing.T) {
	stub := dependabotFileStub(404)
	res := run(t, &DependabotFile{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 1 {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}
