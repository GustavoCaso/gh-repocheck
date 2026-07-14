package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

// errStdinClosed aborts the interactive flow when input runs out.
var errStdinClosed = errors.New("stdin closed")

// RunInit interactively builds a policy and writes it to path.
// Returns the process exit code.
func RunInit(path string, out io.Writer, in io.Reader) int {
	if path == "" {
		fmt.Fprintln(out, "could not determine user config directory")
		return exitUsage
	}
	p := &prompter{r: bufio.NewReader(in), w: out}

	if _, err := os.Stat(path); err == nil {
		overwrite, overwriteErr := p.boolQ(fmt.Sprintf("%s exists; overwrite?", path), false)
		if overwriteErr != nil || !overwrite {
			fmt.Fprintln(out, "aborted")
			if overwriteErr != nil {
				return 1
			}
			return 0
		}
	} else {
		fmt.Fprintf(out, "creating policy at: %s\n", path)
	}

	pol, buildErr := buildPolicy(p)
	if buildErr != nil {
		fmt.Fprintln(out, "aborted")
		return 1
	}

	if err := writePolicy(path, pol); err != nil {
		fmt.Fprintln(out, err)
		return exitUsage
	}
	fmt.Fprintf(out, "wrote %s\n", path)
	return 0
}

func buildPolicy(p *prompter) (policy.Policy, error) {
	pol := policy.Defaults()
	c := &pol.Checks

	var err error
	ask := func(q string, def bool) bool {
		if err != nil {
			return false
		}
		var v bool
		v, err = p.boolQ(q, def)
		return v
	}

	if c.SecretScanning.Enabled = ask("secret-scanning: enable?", c.SecretScanning.Enabled); c.SecretScanning.Enabled {
		c.SecretScanning.PushProtection = ask("secret-scanning: push protection?", c.SecretScanning.PushProtection)
	} else {
		c.SecretScanning.PushProtection = false
	}
	c.CodeQL.Enabled = ask("codeql: enable?", c.CodeQL.Enabled)
	c.Dependabot.Enabled = ask("dependabot: enable?", c.Dependabot.Enabled)
	c.DependabotFile.Enabled = ask("dependabot_file: enable?", c.DependabotFile.Enabled)
	if c.License.Enabled = ask("license: enable?", c.License.Enabled); c.License.Enabled && err == nil {
		c.License.Allowed, err = p.listQ("license: allowed SPDX ids (comma-separated, empty = any)", nil)
	}
	//nolint:nestif // the complexity is acceptable
	if c.Rulesets.Enabled = ask("rulesets: enable?", c.Rulesets.Enabled); c.Rulesets.Enabled {
		r := &c.Rulesets.Rules
		r.BlockForcePush = ask("rulesets: block force push?", r.BlockForcePush)
		r.BlockDeletion = ask("rulesets: block deletion?", r.BlockDeletion)
		r.RequireSignatures = ask("rulesets: require signed commits?", r.RequireSignatures)
		r.RequireLinearHistory = ask("rulesets: require linear history?", r.RequireLinearHistory)
		if r.RequirePR = ask("rulesets: require pull requests?", r.RequirePR); r.RequirePR && err == nil {
			r.RequiredApprovals, err = p.intQ("rulesets: required approvals", r.RequiredApprovals)
			r.DismissStaleReviews = ask("rulesets: dismiss stale reviews?", r.DismissStaleReviews)
			r.RequireCodeOwnerReview = ask("rulesets: require code owner review?", r.RequireCodeOwnerReview)
			r.RequireLastPushApproval = ask("rulesets: require last push approval?", r.RequireLastPushApproval)
			r.RequireThreadResolution = ask("rulesets: require thread resolution?", r.RequireThreadResolution)
			if err == nil {
				r.AllowedMergeMethods, err = p.mergeMethodsQ()
			}
		}
		if err == nil {
			r.RequiredStatusChecks, err = p.listQ(
				"rulesets: required status checks (comma-separated, empty = none)",
				nil,
			)
		}
		if len(r.RequiredStatusChecks) > 0 {
			r.StrictStatusChecks = ask(
				"rulesets: strict status checks (require branches up to date)?",
				r.StrictStatusChecks,
			)
		}
	} else {
		c.Rulesets.Rules = policy.RulesetRules{}
	}
	return pol, err
}

func writePolicy(path string, pol policy.Policy) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := yaml.Marshal(pol)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

type prompter struct {
	r *bufio.Reader
	w io.Writer
}

func (p *prompter) line(prompt string) (string, error) {
	fmt.Fprint(p.w, prompt)
	line, err := p.r.ReadString('\n')
	if err != nil && line == "" {
		return "", errStdinClosed
	}
	return strings.TrimSpace(line), nil
}

// boolQ asks a yes/no question; empty answer returns def, invalid input re-asks.
func (p *prompter) boolQ(q string, def bool) (bool, error) {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	for {
		ans, err := p.line(fmt.Sprintf("%s %s ", q, hint))
		if err != nil {
			return false, err
		}
		switch strings.ToLower(ans) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		fmt.Fprintln(p.w, "please answer y or n")
	}
}

// intQ asks for a non-negative integer; empty answer returns def.
func (p *prompter) intQ(q string, def int) (int, error) {
	for {
		ans, err := p.line(fmt.Sprintf("%s [%d]: ", q, def))
		if err != nil {
			return 0, err
		}
		if ans == "" {
			return def, nil
		}
		n, convErr := strconv.Atoi(ans)
		if convErr == nil && n >= 0 {
			return n, nil
		}
		fmt.Fprintln(p.w, "please enter a non-negative number")
	}
}

// listQ asks for a comma-separated list; empty answer returns def.
func (p *prompter) listQ(q string, def []string) ([]string, error) {
	ans, err := p.line(fmt.Sprintf("%s: ", q))
	if err != nil {
		return nil, err
	}
	if ans == "" {
		return def, nil
	}
	var items []string
	for s := range strings.SplitSeq(ans, ",") {
		if s = strings.TrimSpace(s); s != "" {
			items = append(items, s)
		}
	}
	return items, nil
}

// mergeMethodsQ asks for allowed merge methods, re-asking on unknown values.
func (p *prompter) mergeMethodsQ() ([]policy.MergeType, error) {
	for {
		items, err := p.listQ(
			"rulesets: allowed merge methods (merge/squash/rebase, comma-separated, empty = any)",
			nil,
		)
		if err != nil {
			return nil, err
		}
		methods := make([]policy.MergeType, 0, len(items))
		valid := true
		for _, s := range items {
			switch m := policy.MergeType(strings.ToLower(s)); m {
			case policy.MergeMethod, policy.SquashMethod, policy.RebaseMethod:
				methods = append(methods, m)
			default:
				fmt.Fprintf(p.w, "unknown merge method %q; use merge, squash, or rebase\n", s)
				valid = false
			}
		}
		if valid {
			if len(methods) == 0 {
				return nil, nil
			}
			return methods, nil
		}
	}
}
