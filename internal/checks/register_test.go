package checks

import (
	"testing"

	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func TestDefaultRegistryHasAllChecks(t *testing.T) {
	defaultChecks := []string{"codeql", "configuration", "dependabot", "dependabot-file", "license", "rulesets", "secret-scanning"}
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

func TestChecksHonorPolicyEnabled(t *testing.T) {
	off := policy.Defaults()
	off.Checks.SecretScanning.Enabled = false
	off.Checks.CodeQL.Enabled = false
	off.Checks.Configuration.Enabled = false
	off.Checks.Dependabot.Enabled = false
	off.Checks.DependabotFile.Enabled = false
	off.Checks.License.Enabled = false
	off.Checks.Rulesets.Enabled = false

	on := policy.Defaults()
	on.Checks.DependabotFile.Enabled = true // off in Defaults()
	on.Checks.Configuration.Enabled = true  // off in Defaults()

	for _, c := range DefaultRegistry().All() {
		if c.Enabled(off) {
			t.Errorf("%s: Enabled = true, want false when disabled by policy", c.ID())
		}
		if !c.Enabled(on) {
			t.Errorf("%s: Enabled = false, want true when enabled by policy", c.ID())
		}
	}
}
