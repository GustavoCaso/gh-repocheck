package policy

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	p := Defaults()
	if !p.Checks.SecretScanning.Enabled {
		t.Error("secret-scanning should default enabled")
	}
	if !p.Checks.SecretScanning.PushProtection {
		t.Error("push-protection should default true")
	}
	if !p.Checks.Rulesets.Rules.BlockForcePush || !p.Checks.Rulesets.Rules.BlockDeletion {
		t.Error("ruleset force-push/deletion blocks should default true")
	}
	if p.Checks.Rulesets.Rules.RequirePR {
		t.Error("require-pr should default false")
	}
	if !p.Checks.Dependabot.Enabled {
		t.Error("dependabot should default enabled")
	}
	if p.Checks.DependabotFile.Enabled {
		t.Error("dependabot-file should default not enabled")
	}
	if p.Checks.Configuration.Enabled {
		t.Error("configuration should default not enabled")
	}
	if len(p.Checks.License.Allowed) != 0 {
		t.Error("license allowed list should default empty")
	}
}

func TestParseOverridesDefaults(t *testing.T) {
	yml := `
checks:
  codeql:
    enabled: false
  license:
    allowed: [MIT, Apache-2.0]
  rulesets:
    rules:
      require-pr: true
      required-approvals: 2
`
	p, err := Parse(strings.NewReader(yml))
	if err != nil {
		t.Fatal(err)
	}
	if p.Checks.CodeQL.Enabled {
		t.Error("codeql should be disabled")
	}
	if !p.Checks.SecretScanning.Enabled {
		t.Error("secret-scanning should stay enabled")
	}
	if got := p.Checks.License.Allowed; len(got) != 2 || got[0] != "MIT" {
		t.Errorf("license allowed = %v", got)
	}
	if !p.Checks.Rulesets.Rules.RequirePR || p.Checks.Rulesets.Rules.RequiredApprovals != 2 {
		t.Error("ruleset overrides not applied")
	}
}

func TestParseEmptyInputReturnsDefaults(t *testing.T) {
	p, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse(empty) error = %v", err)
	}
	if !reflect.DeepEqual(p, Defaults()) {
		t.Error("Parse(empty) should return defaults")
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse(strings.NewReader("checks:\n  secret-scannning:\n    enabled: true\n"))
	if err == nil {
		t.Fatal("expected error for unknown key (typo), got nil")
	}
}
