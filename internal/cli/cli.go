package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/checks"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
	"github.com/GustavoCaso/gh-repocheck/internal/runner"
)

type Options struct {
	Repo            string
	Owner           string
	DryRun          bool
	Fix             bool
	Checks          []string
	Format          string
	PolicyPath      string
	IncludeArchived bool
	IncludeForks    bool
	List            bool
}

func ParseArgs(args []string) (Options, error) {
	var opts Options
	if len(args) > 0 && args[0] == "list" {
		opts.List = true
		return opts, nil
	}
	fs := flag.NewFlagSet("gh repocheck", flag.ContinueOnError)
	fs.StringVar(&opts.Repo, "repo", "", "target repository (owner/name); default: current directory's repo")
	fs.StringVar(&opts.Owner, "owner", "", "check every repo owned by this user/org")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "report findings without prompting or fixing")
	fs.BoolVar(&opts.Fix, "fix", false, "apply all fixes without prompting")
	checksFlag := fs.String("checks", "", "comma-separated subset of checks to run")
	fs.StringVar(&opts.Format, "format", "human", "output format: human or json")
	fs.StringVar(&opts.PolicyPath, "policy", "", "path to a policy YAML file")
	fs.BoolVar(&opts.IncludeArchived, "include-archived", false, "include archived repos in --owner sweeps")
	fs.BoolVar(&opts.IncludeForks, "include-forks", false, "include forks in --owner sweeps")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if *checksFlag != "" {
		opts.Checks = strings.Split(*checksFlag, ",")
	}
	if opts.Fix && opts.DryRun {
		return opts, errors.New("--fix and --dry-run are mutually exclusive")
	}
	if opts.Repo != "" && !strings.Contains(opts.Repo, "/") {
		return opts, fmt.Errorf("--repo must be owner/name, got %q", opts.Repo)
	}
	if opts.Format != "human" && opts.Format != "json" {
		return opts, fmt.Errorf("--format must be human or json, got %q", opts.Format)
	}
	if opts.Fix && opts.Format == "json" {
		return opts, errors.New("--fix requires human format (interactive fixing is not supported with --format json)")
	}
	return opts, nil
}

// Run is the entry point called by main. Returns the process exit code:
// 0 clean, 1 findings or check errors, 2 usage/infrastructure errors.
func Run(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	opts, err := ParseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	reg := checks.DefaultRegistry()

	if opts.List {
		for _, c := range reg.All() {
			fixable := ""
			if _, ok := c.(check.Fixable); ok {
				fixable = " (fixable)"
			}
			fmt.Fprintf(stdout, "%-18s %s%s\n", c.ID(), c.Description(), fixable)
		}
		return 0
	}

	selected := reg.All()
	if len(opts.Checks) > 0 {
		selected, err = reg.Select(opts.Checks)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}

	rest, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(stderr, "could not create GitHub client (is gh authenticated?):", err)
		return 2
	}
	client := &githubapi.GH{REST: rest}
	ctx := context.Background()

	repos, err := resolveRepos(ctx, client, opts, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	pol, polSrc, err := resolvePolicy(ctx, client, opts, repos)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.Format == "human" && polSrc != "defaults" {
		fmt.Fprintf(stdout, "policy: %s\n\n", polSrc)
	}

	grouped := runner.RunRepos(ctx, client, selected, repos, pol)
	var results []runner.CheckResult
	for _, g := range grouped {
		results = append(results, g...)
	}

	if opts.Format == "json" {
		if err := RenderJSON(stdout, results); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	} else {
		RenderHuman(stdout, results)
	}

	if !opts.DryRun && opts.Format == "human" {
		fmt.Fprintln(stdout)
		ApplyFixes(ctx, client, results, pol, stdout, stdin, opts.Fix)
	}

	if HasFailures(results) {
		return 1
	}
	return 0
}

func resolveRepos(ctx context.Context, client githubapi.Client, opts Options, errw io.Writer) ([]check.Repo, error) {
	switch {
	case opts.Owner != "":
		var login struct {
			Login string `json:"login"`
		}
		if err := client.Get(ctx, "user", &login); err == nil {
			if strings.EqualFold(login.Login, opts.Owner) {
				return githubapi.ListViewerRepos(ctx, client, opts.IncludeArchived, opts.IncludeForks)
			}
		} else {
			fmt.Fprintf(errw, "warning: could not determine viewer identity (%v); falling back to org/user repo listing\n", err)
		}
		// Try the org listing first: users/{owner}/repos only returns public
		// repos for organizations. Fall back to the user listing on 404.
		repos, err := githubapi.ListOrgRepos(ctx, client, opts.Owner, opts.IncludeArchived, opts.IncludeForks)
		if err != nil {
			if githubapi.StatusCode(err) == http.StatusNotFound {
				return githubapi.ListOwnerRepos(ctx, client, opts.Owner, opts.IncludeArchived, opts.IncludeForks)
			}
			return nil, fmt.Errorf("listing repos for %s: %w", opts.Owner, err)
		}
		return repos, nil
	case opts.Repo != "":
		parts := strings.SplitN(opts.Repo, "/", 2)
		repo, err := githubapi.FetchRepo(ctx, client, parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", opts.Repo, err)
		}
		return []check.Repo{repo}, nil
	default:
		current, err := repository.Current()
		if err != nil {
			return nil, errors.New("not in a git repo with a GitHub remote; use --repo owner/name")
		}
		repo, err := githubapi.FetchRepo(ctx, client, current.Owner, current.Name)
		if err != nil {
			return nil, err
		}
		return []check.Repo{repo}, nil
	}
}

// resolvePolicy fetches .github/repocheck.yml only in single-repo mode.
// Any fetch failure (404 or otherwise) falls through to user config/defaults.
func resolvePolicy(ctx context.Context, client githubapi.Client, opts Options, repos []check.Repo) (policy.Policy, string, error) {
	var repoContent []byte
	if opts.PolicyPath == "" && len(repos) == 1 {
		path := fmt.Sprintf("repos/%s/%s/contents/.github/repocheck.yml",
			repos[0].Owner, repos[0].Name)
		var envelope struct {
			Content string `json:"content"`
		}
		if err := client.Get(ctx, path, &envelope); err == nil {
			// The contents API returns the file as base64 with embedded newlines.
			if raw, err := base64.StdEncoding.DecodeString(
				strings.ReplaceAll(envelope.Content, "\n", "")); err == nil {
				repoContent = raw
			}
		}
	}
	return policy.Resolve(opts.PolicyPath, repoContent, policy.UserConfigPath())
}
