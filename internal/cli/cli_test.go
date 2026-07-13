package cli

import "testing"

func TestParseArgs(t *testing.T) {
	opts, err := ParseArgs([]string{"--repo", "o/r", "--dry-run", "--checks", "codeql,license", "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Repo != "o/r" || !opts.DryRun || opts.Format != "json" {
		t.Errorf("opts = %+v", opts)
	}
	if len(opts.Checks) != 2 || opts.Checks[0] != "codeql" {
		t.Errorf("checks = %v", opts.Checks)
	}
}

func TestParseArgsListSubcommand(t *testing.T) {
	opts, err := ParseArgs([]string{"list"})
	if err != nil || !opts.List {
		t.Errorf("opts = %+v, err = %v", opts, err)
	}
}

func TestParseArgsRejectsFixWithDryRun(t *testing.T) {
	if _, err := ParseArgs([]string{"--fix", "--dry-run"}); err == nil {
		t.Error("--fix with --dry-run should error")
	}
}

func TestParseArgsRejectsBadRepo(t *testing.T) {
	if _, err := ParseArgs([]string{"--repo", "not-a-repo"}); err == nil {
		t.Error("--repo without owner/ should error")
	}
}

func TestParseArgsRejectsFixWithJSONFormat(t *testing.T) {
	if _, err := ParseArgs([]string{"--fix", "--format", "json"}); err == nil {
		t.Error("--fix with --format json should error")
	}
}

func TestParseArgsRejectsBadFormat(t *testing.T) {
	if _, err := ParseArgs([]string{"--format", "xml"}); err == nil {
		t.Error("--format xml should error")
	}
}
