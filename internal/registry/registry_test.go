package registry

import (
	"context"
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type fake struct{ id string }

func (f *fake) ID() string          { return f.id }
func (f *fake) Description() string { return "fake" }
func (f *fake) Run(context.Context, githubapi.Client, check.Repo, policy.Policy) (check.Result, error) {
	return check.Result{Status: check.Pass}, nil
}

func TestRegisterAndAll(t *testing.T) {
	r := New()
	r.Register(&fake{id: "b"})
	r.Register(&fake{id: "a"})
	all := r.All()
	if len(all) != 2 || all[0].ID() != "a" || all[1].ID() != "b" {
		t.Errorf("All() should return checks sorted by ID, got %v", ids(all))
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	r := New()
	r.Register(&fake{id: "a"})
	r.Register(&fake{id: "a"})
}

func TestSelectSubset(t *testing.T) {
	r := New()
	r.Register(&fake{id: "a"})
	r.Register(&fake{id: "b"})
	sel, err := r.Select([]string{"b"})
	if err != nil || len(sel) != 1 || sel[0].ID() != "b" {
		t.Errorf("Select = %v, %v", ids(sel), err)
	}
	if _, selErr := r.Select([]string{"nope"}); selErr == nil {
		t.Error("unknown check id should error")
	}
}

func ids(cs []check.Check) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID()
	}
	return out
}
