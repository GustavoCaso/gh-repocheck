package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveFlagWins(t *testing.T) {
	dir := t.TempDir()
	flagPath := writeFile(t, dir, "flag.yml", "checks:\n  codeql:\n    enabled: false\n")
	userPath := writeFile(t, dir, "user.yml", "checks:\n  license:\n    enabled: false\n")

	p, src, err := Resolve(flagPath, nil, userPath)
	if err != nil {
		t.Fatal(err)
	}
	if p.Checks.CodeQL.Enabled {
		t.Error("flag policy not applied")
	}
	if !p.Checks.License.Enabled {
		t.Error("user policy should not apply when flag given")
	}
	if src != flagPath {
		t.Errorf("source = %q", src)
	}
}

func TestResolveRepoContentBeatsUserFile(t *testing.T) {
	dir := t.TempDir()
	userPath := writeFile(t, dir, "user.yml", "checks:\n  license:\n    enabled: false\n")
	repoYAML := []byte("checks:\n  codeql:\n    enabled: false\n")

	p, src, err := Resolve("", repoYAML, userPath)
	if err != nil {
		t.Fatal(err)
	}
	if p.Checks.CodeQL.Enabled || !p.Checks.License.Enabled {
		t.Error("repo policy should win over user file")
	}
	if src != ".github/repocheck.yml" {
		t.Errorf("source = %q", src)
	}
}

func TestResolveMalformedFlagFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	flagPath := writeFile(t, dir, "flag.yml", "checks: [not a mapping\n")
	if _, _, err := Resolve(flagPath, nil, ""); err == nil {
		t.Fatal("expected parse error from malformed flag file, got nil")
	}
}

func TestResolveMalformedRepoContentReturnsError(t *testing.T) {
	if _, _, err := Resolve("", []byte("checks: [not a mapping\n"), ""); err == nil {
		t.Fatal("expected parse error from malformed repo content, got nil")
	}
}

func TestResolveDefaultsWhenNothingExists(t *testing.T) {
	p, src, err := Resolve("", nil, filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Checks.SecretScanning.Enabled {
		t.Error("expected defaults")
	}
	if src != "defaults" {
		t.Errorf("source = %q", src)
	}
}
