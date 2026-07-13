# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A `gh` CLI extension that audits GitHub repositories against a security policy (secret scanning, CodeQL, Dependabot, license, branch-protection rulesets) and can fix failures via the GitHub API.

## Commands

```sh
go build ./...                                    # build
go test ./...                                     # all tests
go test ./internal/checks/ -run TestRulesetsFix   # single test
gofmt -l . && go vet ./...                        # format check + vet
go build -o gh-repocheck && gh extension install .  # install locally for manual runs
```

Tests need no network or auth — all GitHub API access in tests goes through `githubapi.Stub`.

## Architecture

Data flow: `main.go` → `internal/cli.Run` (flag parsing, repo resolution, output rendering, interactive fix prompt) → `internal/runner` (executes checks concurrently per repo) → individual checks in `internal/checks/`.

- **`internal/check`** — core interfaces. Every check implements `check.Check` (`ID`, `Description`, `Run`); checks that can remediate also implement `check.Fixable` (`Fix`). `Run` returns a `Result` with `Status` (Pass/Fail/Warn/Skip) and `Findings` (each with a `Message` and, if fixable, a `FixHint`).
- **`internal/checks`** — the built-in checks, one file each with a sibling `_test.go`. To add a check: implement the interface here and register it in `register.go`'s `DefaultRegistry()`.
- **`internal/githubapi`** — `Client` interface (Get/Post/Patch/Put) abstracting go-gh's REST client so checks are testable. `Stub` is the test double: map of `"METHOD path"` → canned JSON response; it records mutating requests for assertions and returns 404 for unstubbed paths. `StatusCode(err)` extracts HTTP status from errors.
- **`internal/policy`** — desired-state config, YAML-parsed over `Defaults()` with unknown keys rejected. `Resolve` picks the policy: `--policy` flag → repo's `.github/repocheck.yml` → `~/.config/gh-repocheck/policy.yml` → defaults. Policy YAML keys are kebab-case.
- **`internal/registry`** / **`internal/runner`** — check registration and concurrent execution; `runner.Enabled` gates checks disabled by policy.

## Conventions

- Checks must degrade gracefully: API 404s that mean "feature unavailable" (e.g. GHAS missing, org-inherited rulesets not inspectable at repo level) produce Skip or are ignored, not errors.
- Test helpers `run`, `runWithPolicy`, and `testRepo` (owner `o`, repo `r`, default branch `main`) live in `secretscanning_test.go` and are shared across the package.
- The README documents every policy option with a full example under "## Policy" — keep it in sync when adding policy fields.
- The `gh-repocheck` binary at the repo root is a build artifact; don't commit it.
