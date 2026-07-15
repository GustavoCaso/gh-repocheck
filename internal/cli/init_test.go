package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

func initPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "gh-repocheck", "policy.yml")
}

// answers joins prompt answers with newlines; empty strings accept defaults.
func answers(lines ...string) *strings.Reader {
	return strings.NewReader(strings.Join(lines, "\n") + "\n")
}

func parseWritten(t *testing.T, path string) policy.Policy {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening written policy: %v", err)
	}
	defer f.Close()
	p, err := policy.Parse(f)
	if err != nil {
		t.Fatalf("written policy does not re-parse: %v", err)
	}
	return p
}

func TestInitAllDefaults(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	// Enough blank lines to accept every default; extra blanks are harmless
	// only if the flow stops asking, so give exactly the expected count:
	// 7 enables + push-protection + license-allowed + 4 ruleset bools +
	// require-pr + status-checks = 15.
	in := answers("", "", "", "", "", "", "", "", "", "", "", "", "", "", "")
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	got := parseWritten(t, path)
	want := policy.Defaults()
	// Compare YAML forms: re-parsing turns absent slices into empty ones,
	// which DeepEqual would flag despite being semantically identical.
	gotY, _ := yaml.Marshal(got)
	wantY, _ := yaml.Marshal(want)
	if string(gotY) != string(wantY) {
		t.Errorf("policy = %s, want defaults %s", gotY, wantY)
	}
	if !strings.Contains(out.String(), "wrote "+path) {
		t.Errorf("output missing 'wrote' confirmation:\n%s", out.String())
	}
}

func TestInitDisabledCheckSkipsOptions(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	// Disable everything: 7 "n" answers, no sub-questions expected.
	in := answers("n", "n", "n", "n", "n", "n", "n")
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if strings.Contains(out.String(), "push protection") {
		t.Errorf("asked push protection for disabled secret-scanning:\n%s", out.String())
	}
	got := parseWritten(t, path)
	if got.Checks.SecretScanning.Enabled || got.Checks.Rulesets.Enabled {
		t.Errorf("checks not disabled: %+v", got.Checks)
	}
}

func TestInitRequirePRAsksSubOptions(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	// Disable all checks except rulesets; enable require-pr with 2 approvals,
	// squash-only merges, one required status check with strict mode.
	in := answers(
		"n",            // secret-scanning
		"n",            // codeql
		"n",            // configuration
		"n",            // dependabot
		"n",            // dependabot_file
		"n",            // license
		"y",            // rulesets
		"",             // block-force-push (default true)
		"",             // block-deletion (default true)
		"y",            // require-signatures
		"",             // require-linear-history (default false)
		"y",            // require-pr
		"2",            // required-approvals
		"y",            // dismiss-stale-reviews
		"",             // require-code-owner-review
		"",             // require-last-push-approval
		"y",            // require-thread-resolution
		"squash",       // allowed-merge-methods
		"ci/test,lint", // required-status-checks
		"y",            // strict-status-checks
	)
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	r := parseWritten(t, path).Checks.Rulesets.Rules
	if !r.RequirePR || r.RequiredApprovals != 2 || !r.DismissStaleReviews ||
		!r.RequireSignatures || !r.RequireThreadResolution || !r.StrictStatusChecks {
		t.Errorf("ruleset rules = %+v", r)
	}
	if len(r.AllowedMergeMethods) != 1 || r.AllowedMergeMethods[0] != policy.SquashMethod {
		t.Errorf("allowed merge methods = %v, want [squash]", r.AllowedMergeMethods)
	}
	if len(r.RequiredStatusChecks) != 2 || r.RequiredStatusChecks[0] != "ci/test" {
		t.Errorf("required status checks = %v", r.RequiredStatusChecks)
	}
}

func TestInitNoPRSkipsSubOptions(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	in := answers(
		"n", "n", "n", "n", "n", "n", // other checks off
		"y",            // rulesets
		"", "", "", "", // 4 ruleset bools default
		"", // require-pr default no
		"", // required-status-checks empty
	)
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	if strings.Contains(out.String(), "approvals") {
		t.Errorf("asked approvals without require-pr:\n%s", out.String())
	}
	if strings.Contains(out.String(), "strict") {
		t.Errorf("asked strict-status-checks without status checks:\n%s", out.String())
	}
}

func TestInitInvalidInputReasks(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	in := answers(
		"maybe", "y", // secret-scanning re-ask
		"",  // push-protection
		"",  // codeql
		"",  // configuration (default no)
		"",  // dependabot
		"",  // dependabot_file
		"n", // license
		"y", // rulesets
		"", "", "", "",
		"y",        // require-pr
		"two", "3", // approvals: invalid then valid
		"", "", "", "",
		"merge,teleport", "rebase", // merge methods: invalid then valid
		"", // status checks
	)
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	r := parseWritten(t, path).Checks.Rulesets.Rules
	if r.RequiredApprovals != 3 {
		t.Errorf("approvals = %d, want 3", r.RequiredApprovals)
	}
	if len(r.AllowedMergeMethods) != 1 || r.AllowedMergeMethods[0] != policy.RebaseMethod {
		t.Errorf("merge methods = %v, want [rebase]", r.AllowedMergeMethods)
	}
}

func TestInitConfigurationAsksSubOptions(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	in := answers(
		"n", // secret-scanning
		"n", // codeql
		"y", // configuration
		"",  // has-issues (default true)
		"n", // has-projects
		"n", // has-wiki
		"",  // allow-squash-merge (default true)
		"y", // allow-merge-commit
		"",  // allow-rebase-merge (default true)
		"y", // allow-auto-merge
		"y", // delete-branch-on-merge
		"",  // allow-forking (default false)
		"",  // web-commit-signoff-required (default false)
		"n", // dependabot
		"n", // dependabot_file
		"n", // license
		"n", // rulesets
	)
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	c := parseWritten(t, path).Checks.Configuration
	if !c.Enabled || !c.HasIssues || c.HasProjects || c.HasWiki ||
		!c.AllowSquashMerge || !c.AllowMergeCommit || !c.AllowRebaseMerge ||
		!c.AllowAutoMerge || !c.DeleteBranchOnMerge || c.AllowForking ||
		c.WebCommitSignoffRequired {
		t.Errorf("configuration = %+v", c)
	}
}

func TestInitExistingFileDeclineOverwrite(t *testing.T) {
	path := initPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("checks:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := RunInit(path, &out, answers("")) // default: no overwrite
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Errorf("output missing 'aborted':\n%s", out.String())
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "checks:\n" {
		t.Errorf("existing file modified: %q, %v", content, err)
	}
}

func TestInitExistingFileAcceptOverwrite(t *testing.T) {
	path := initPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	in := answers("y", "n", "n", "n", "n", "n", "n", "n") // overwrite + disable all
	code := RunInit(path, &out, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	got := parseWritten(t, path)
	if got.Checks.CodeQL.Enabled {
		t.Errorf("file not overwritten: %+v", got.Checks)
	}
}

func TestInitEOFAborts(t *testing.T) {
	path := initPath(t)
	var out bytes.Buffer
	code := RunInit(path, &out, strings.NewReader("n\nn\n")) // stdin ends mid-flow
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file written despite EOF abort")
	}
}

func TestParseArgsInit(t *testing.T) {
	opts, err := ParseArgs([]string{"init"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Init {
		t.Error("Init not set for 'init' subcommand")
	}
}
