package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
	"github.com/GustavoCaso/gh-repocheck/internal/runner"
)

// ApplyFixes walks failed fixable results. In auto mode it fixes everything;
// otherwise it prompts y/n/a/q per finding. Returns the number of fixes applied.
func ApplyFixes(ctx context.Context, client githubapi.Client, results []runner.CheckResult,
	pol policy.Policy, out io.Writer, in io.Reader, auto bool) int {

	reader := bufio.NewReader(in)
	fixed := 0
	fixAll := auto
	for _, r := range results {
		if r.Err != nil || r.Result.Status != check.Fail {
			continue
		}
		fixable, ok := r.Check.(check.Fixable)
		if !ok {
			continue
		}
		if !fixAll {
			hint := ""
			if len(r.Result.Findings) > 0 && r.Result.Findings[0].FixHint != "" {
				hint = " (" + r.Result.Findings[0].FixHint + ")"
			}
			fmt.Fprintf(out, "Fix %s on %s%s? [y/n/a/q] ", r.Check.ID(), r.Repo.FullName(), hint)
			line, err := reader.ReadString('\n')
			if err != nil && line == "" {
				return fixed // stdin closed: stop prompting
			}
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "y":
			case "a":
				fixAll = true
			case "q":
				return fixed
			default: // n or anything else
				continue
			}
		}
		if err := fixable.Fix(ctx, client, r.Repo, pol); err != nil {
			fmt.Fprintf(out, "  failed to fix %s on %s: %v\n", r.Check.ID(), r.Repo.FullName(), err)
			continue
		}
		fixed++
		fmt.Fprintf(out, "  fixed %s on %s\n", r.Check.ID(), r.Repo.FullName())
	}
	return fixed
}
