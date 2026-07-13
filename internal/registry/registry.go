// Package registry holds the set of available checks.
package registry

import (
	"fmt"
	"sort"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
)

type Registry struct {
	checks map[string]check.Check
}

func New() *Registry {
	return &Registry{checks: map[string]check.Check{}}
}

func (r *Registry) Register(c check.Check) {
	if _, dup := r.checks[c.ID()]; dup {
		panic("duplicate check id: " + c.ID())
	}
	r.checks[c.ID()] = c
}

// All returns every check sorted by ID for stable output.
func (r *Registry) All() []check.Check {
	out := make([]check.Check, 0, len(r.checks))
	for _, c := range r.checks {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Select returns the checks with the given ids, erroring on unknown ids.
func (r *Registry) Select(ids []string) ([]check.Check, error) {
	out := make([]check.Check, 0, len(ids))
	for _, id := range ids {
		c, ok := r.checks[id]
		if !ok {
			return nil, fmt.Errorf("unknown check %q (run 'gh repocheck list')", id)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out, nil
}
