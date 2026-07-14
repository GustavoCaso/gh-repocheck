package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type rc struct{ id string }

func (r *rc) ID() string                 { return r.id }
func (r *rc) Description() string        { return r.id }
func (r *rc) Enabled(policy.Policy) bool { return true }
func (r *rc) Run(context.Context, githubapi.Client, check.Repo, policy.Policy) check.Result {
	return check.Result{}
}

func sample() []check.Result {
	repo := check.Repo{Owner: "o", Name: "r"}
	return []check.Result{
		{Repo: repo, Check: &rc{"secret-scanning"}, Status: check.Pass},
		{Repo: repo, Check: &fixableCheck{rc: rc{id: "codeql"}}, Status: check.Fail,
			Findings: []check.Finding{{Message: "not enabled", FixHint: "enable it"}}},
		{Repo: repo, Check: &rc{"license"}, Status: check.Skip,
			Findings: []check.Finding{{Message: "disabled by policy"}}},
		{Repo: repo, Check: &rc{"dependabot"}, Error: errors.New("boom")},
	}
}

func TestRenderHuman(t *testing.T) {
	var buf bytes.Buffer
	RenderHuman(&buf, sample())
	out := buf.String()
	for _, want := range []string{"o/r", "✓ secret-scanning", "✗ codeql", "not enabled", "- license", "! dependabot", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[1]["check"] != "codeql" || rows[1]["status"] != "fail" || rows[1]["fixable"] != true {
		t.Errorf("row = %v", rows[1])
	}
}

func TestHasFailures(t *testing.T) {
	if !HasFailures(sample()) {
		t.Error("sample has failures")
	}
	if HasFailures(sample()[:1]) {
		t.Error("pass-only should not report failures")
	}
}
