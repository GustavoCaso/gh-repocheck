# gh-repocheck

A [GitHub CLI](https://cli.github.com) extension that audits repositories
against a security and hygiene policy — and can fix what it finds.

It checks secret scanning, CodeQL, Dependabot, license presence,
branch-protection rulesets, and repository settings, reports pass/fail per
repo, and (interactively or
automatically) enables the missing settings via the GitHub API.

## Install

```sh
gh extension install GustavoCaso/gh-repocheck
```

Requires an authenticated `gh` (`gh auth login`).

## Usage

```sh
gh repocheck [flags]
gh repocheck list
gh repocheck init
```

With no flags, it checks the repository of the current directory's GitHub
remote, prints findings, and prompts `[y/n/a/q]` for each fixable failure
(`y` fix this one, `n` skip, `a` fix this and all remaining, `q` quit).

### Examples

```sh
# Audit the current directory's repo, prompting for fixes
gh repocheck

# Audit a specific repo without touching anything
gh repocheck --repo cli/cli --dry-run

# Apply every available fix without prompting
gh repocheck --repo myorg/myrepo --fix

# Sweep every repo owned by a user or organization
gh repocheck --owner myorg
gh repocheck --owner myorg --include-archived --include-forks

# Run only some checks
gh repocheck --checks codeql,license

# Machine-readable output (implies no fix prompting)
gh repocheck --repo cli/cli --format json

# Use an explicit policy file
gh repocheck --policy ./policy.yml

# Show the available checks
gh repocheck list

# Interactively create ~/.config/gh-repocheck/policy.yml
gh repocheck init
```

### Flags

| Flag | Description |
|---|---|
| `--repo owner/name` | Target repository. Default: the current directory's GitHub remote. |
| `--owner name` | Check every repo owned by this user or org (orgs are tried first; private org repos require token access). |
| `--dry-run` | Report findings only; never prompt or fix. Mutually exclusive with `--fix`. |
| `--fix` | Apply all fixes without prompting. |
| `--checks a,b,c` | Comma-separated subset of checks to run (ids from `gh repocheck list`). |
| `--format human\|json` | Output format (default `human`). JSON mode never prompts or fixes. |
| `--policy path` | Path to a policy YAML file (overrides repo and user policies). |
| `--include-archived` | Include archived repos in `--owner` sweeps. |
| `--include-forks` | Include forks in `--owner` sweeps. |

## Checks

| ID | Verifies | Fixable |
|---|---|---|
| `secret-scanning` | Secret scanning and push protection are enabled | yes |
| `codeql` | CodeQL default setup is enabled | yes |
| `configuration` | Repository settings (issues, projects, wiki, merge strategies, auto-merge, branch deletion on merge, forking, web commit signoff) match the policy. Disabled by default | yes |
| `dependabot` | Dependabot vulnerability alerts and automated security fixes are enabled; warns if `.github/dependabot.yml` is missing | yes |
| `dependabot-file` | warns if `.github/dependabot.yml` is missing | no |
| `license` | Repository has a license (optionally from an allowed SPDX list) | no — choosing a license is a human decision |
| `rulesets` | An active ruleset protects the default branch (block force-push, block deletion; optionally signed commits, linear history, PRs with review requirements and merge-method restrictions, required status checks). Rule parameters are validated against the policy, not just the rule's presence | yes |

Notes and caveats:

- **Token scopes**: reading needs `repo` access to the target repositories;
  fixing changes repository settings, so the token must have admin permission
  on the repo (the default `gh auth login` token on your own repos works).
- **GHAS**: on private repos without GitHub Advanced Security, secret scanning
  and CodeQL are unavailable — those checks report `skip`, not `fail`.
- **CodeQL**: skipped when the repo has no CodeQL-supported languages.
- **Configuration**: the GitHub API only returns the merge-strategy settings
  (`allow_squash_merge` etc.) when the token has push access to the repo;
  without it those settings read as `false` and may report spurious failures.
- **Org rulesets**: rulesets inherited from the organization appear in the
  repo's ruleset list but cannot be inspected through the repo-level API; they
  are ignored, so a branch protected only by an org ruleset may report as
  unprotected here.

## Policy

Policy resolution order (first match wins):

1. `--policy path` flag
2. `.github/repocheck.yml` in the target repo (single-repo mode only)
3. `~/.config/gh-repocheck/policy.yml` (`os.UserConfigDir`)
4. Built-in defaults

`gh repocheck init` walks through every option interactively (Enter accepts
the shown default, disabled checks skip their sub-options) and writes the
result to the user config path. If a file already exists there, it asks
before overwriting.

A policy file overrides the defaults key-by-key; unknown keys are an error.
Full example (values shown are the built-in defaults, except where noted):

```yaml
checks:
  secret-scanning:
    enabled: true
    push-protection: true
  codeql:
    enabled: true
  configuration:
    enabled: false               # opt-in; sub-options only enforced when enabled
    has-issues: true
    has-projects: true
    has-wiki: true
    allow-squash-merge: true
    allow-merge-commit: false
    allow-rebase-merge: true
    allow-auto-merge: false
    delete-branch-on-merge: false
    allow-forking: false
    web-commit-signoff-required: false
  dependabot:
    enabled: true
  dependabot_file:
    enabled: true
  license:
    enabled: true
    allowed: []                  # SPDX ids, e.g. [MIT, Apache-2.0]; empty = any license
  rulesets:
    enabled: true
    rules:
      block-force-push: true
      block-deletion: true
      require-signatures: false
      require-linear-history: false
      # Pull-request rule; sub-options below only enforced when require-pr is true.
      # The rule's parameters are validated against the policy, not just presence.
      require-pr: false
      required-approvals: 0
      dismiss-stale-reviews: false
      require-code-owner-review: false
      require-last-push-approval: false
      require-thread-resolution: false
      allowed-merge-methods: []    # e.g. [squash]; empty = any of merge, squash, rebase
      # Status-check rule; enforced when the list is non-empty
      required-status-checks: []   # check contexts, e.g. [ci/test]
      strict-status-checks: false  # require branches to be up to date before merging
```

Disabled checks report `skip` with "disabled by policy".

## Exit codes

| Code | Meaning |
|---|---|
| 0 | All checks passed, warned, or skipped |
| 1 | At least one check failed or errored |
| 2 | Usage or infrastructure error (bad flags, no auth, unknown check, ...) |

Note: a `--fix` run still exits 1 if findings were present at check time, even
if every one of them was fixed — re-run to verify a clean result.

## Adding a check

1. Implement the `check.Check` interface (`ID`, `Description`, `Run`) in
   `internal/checks/`; optionally implement `check.Fixable` (`Fix`) if the
   failure can be remediated via the API.
2. Register it in `DefaultRegistry` in `internal/checks/register.go`.
3. Policy-driven enablement for built-in checks lives in
   `runner.Enabled`; unregistered/unknown check ids default to enabled.

## Limitations (v1)

- Does not write `.github/dependabot.yml` files (only flags their absence).
- No org-level security settings — repository settings only.
- No GitHub Enterprise Server support.
