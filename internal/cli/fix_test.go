package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type fixableCheck struct {
	rc

	fixed int
}

func (f *fixableCheck) Fix(context.Context, githubapi.Client, check.Repo, policy.Policy) error {
	f.fixed++
	return nil
}

func failResult(c check.Check) check.Result {
	return check.Result{
		Repo:   check.Repo{Owner: "o", Name: "r"},
		Check:  c,
		Status: check.Fail,
		Findings: []check.Finding{
			{Message: "bad", FixHint: "make it good"}},
	}
}

func TestApplyFixesAutoMode(t *testing.T) {
	f := &fixableCheck{rc: rc{id: "codeql"}}
	var out strings.Builder
	n := ApplyFixes(context.Background(), nil, []check.Result{failResult(f)},
		policy.Defaults(), &out, strings.NewReader(""), true /* auto */)
	if n != 1 || f.fixed != 1 {
		t.Errorf("n=%d fixed=%d", n, f.fixed)
	}
}

func TestApplyFixesPromptYesNo(t *testing.T) {
	a := &fixableCheck{rc: rc{id: "a"}}
	b := &fixableCheck{rc: rc{id: "b"}}
	var out strings.Builder
	// y to first, n to second
	n := ApplyFixes(context.Background(), nil,
		[]check.Result{failResult(a), failResult(b)},
		policy.Defaults(), &out, strings.NewReader("y\nn\n"), false)
	if n != 1 || a.fixed != 1 || b.fixed != 0 {
		t.Errorf("n=%d a=%d b=%d", n, a.fixed, b.fixed)
	}
}

func TestApplyFixesAllAndQuit(t *testing.T) {
	a := &fixableCheck{rc: rc{id: "a"}}
	b := &fixableCheck{rc: rc{id: "b"}}
	var out strings.Builder
	// "a" = fix this and all remaining
	n := ApplyFixes(context.Background(), nil,
		[]check.Result{failResult(a), failResult(b)},
		policy.Defaults(), &out, strings.NewReader("a\n"), false)
	if n != 2 {
		t.Errorf("all: n=%d", n)
	}
	// "q" = stop
	a2 := &fixableCheck{rc: rc{id: "a"}}
	b2 := &fixableCheck{rc: rc{id: "b"}}
	n = ApplyFixes(context.Background(), nil,
		[]check.Result{failResult(a2), failResult(b2)},
		policy.Defaults(), &out, strings.NewReader("q\n"), false)
	if n != 0 || b2.fixed != 0 {
		t.Errorf("quit: n=%d", n)
	}
}

func TestApplyFixesSkipsNonFixableAndPasses(t *testing.T) {
	nonFixable := &rc{id: "license"}
	var out strings.Builder
	n := ApplyFixes(context.Background(), nil,
		[]check.Result{failResult(nonFixable)},
		policy.Defaults(), &out, strings.NewReader(""), true)
	if n != 0 {
		t.Errorf("n=%d", n)
	}
}
