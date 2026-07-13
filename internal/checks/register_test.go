package checks

import "testing"

func TestDefaultRegistryHasAllChecks(t *testing.T) {
	all := DefaultRegistry().All()
	want := []string{"codeql", "dependabot", "license", "rulesets", "secret-scanning"}
	if len(all) != len(want) {
		t.Fatalf("got %d checks", len(all))
	}
	for i, c := range all {
		if c.ID() != want[i] {
			t.Errorf("check[%d] = %s, want %s", i, c.ID(), want[i])
		}
	}
}
