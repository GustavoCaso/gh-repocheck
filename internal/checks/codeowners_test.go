package checks

import (
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
)

func codeownersStub(status int, body string) *githubapi.Stub {
	return &githubapi.Stub{Responses: map[string]githubapi.StubResponse{
		"GET repos/o/r/codeowners/errors": {Status: status, Body: body},
	}}
}

func TestCodeownersPass(t *testing.T) {
	stub := codeownersStub(200, `{"errors":[]}`)
	if res := run(t, &Codeowners{}, stub); res.Status != check.Pass {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestCodeownersFailWhenMissing(t *testing.T) {
	stub := codeownersStub(404, "")
	res := run(t, &Codeowners{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 1 {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestCodeownersFailWhenInvalid(t *testing.T) {
	stub := codeownersStub(200, `{"errors":[
		{"line":7,"message":"Invalid pattern on line 7"},
		{"line":12,"message":"Unknown owner on line 12"}
	]}`)
	res := run(t, &Codeowners{}, stub)
	if res.Status != check.Fail || len(res.Findings) != 2 {
		t.Errorf("status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestCodeownersIsNotFixable(t *testing.T) {
	var c check.Check = &Codeowners{}
	if _, ok := c.(check.Fixable); ok {
		t.Error("Codeowners must not be Fixable — assigning code owners is a human decision")
	}
}
