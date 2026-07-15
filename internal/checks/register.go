package checks

import "github.com/GustavoCaso/gh-repocheck/internal/registry"

// DefaultRegistry returns a registry with every built-in check.
// Adding a new check: implement check.Check in this package and add it here.
func DefaultRegistry() *registry.Registry {
	r := registry.New()
	r.Register(&SecretScanning{})
	r.Register(&Dependabot{})
	r.Register(&DependabotFile{})
	r.Register(&CodeQL{})
	r.Register(&License{})
	r.Register(&Rulesets{})
	r.Register(&Configuration{})
	return r
}
