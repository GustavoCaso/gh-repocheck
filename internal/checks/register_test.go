package checks

import "testing"

var defaultChecks = []string{"codeql", "dependabot", "dependabot-file", "license", "rulesets", "secret-scanning"}

func TestDefaultRegistryHasAllChecks(t *testing.T) {
	all := DefaultRegistry().All()
	if len(all) != len(defaultChecks) {
		t.Fatalf("got %d checks", len(all))
	}
	for i, c := range all {
		if c.ID() != defaultChecks[i] {
			t.Errorf("check[%d] = %s, want %s", i, c.ID(), defaultChecks[i])
		}
	}
}
