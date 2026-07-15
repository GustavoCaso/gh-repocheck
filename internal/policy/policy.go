// Package policy holds the desired-state configuration for checks.
package policy

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Policy struct {
	Checks Checks `yaml:"checks"`
}

type Checks struct {
	SecretScanning SecretScanning `yaml:"secret-scanning"`
	CodeQL         CodeQL         `yaml:"codeql"`
	Configuration  Configuration  `yaml:"configuration"`
	Dependabot     Dependabot     `yaml:"dependabot"`
	DependabotFile DependabotFile `yaml:"dependabot_file"`
	License        License        `yaml:"license"`
	Rulesets       Rulesets       `yaml:"rulesets"`
}

type SecretScanning struct {
	Enabled        bool `yaml:"enabled"`
	PushProtection bool `yaml:"push-protection"`
}

type CodeQL struct {
	Enabled bool `yaml:"enabled"`
}

type Configuration struct {
	Enabled                  bool `yaml:"enabled"`
	HasIssues                bool `yaml:"has-issues"`
	HasProjects              bool `yaml:"has-projects"`
	HasWiki                  bool `yaml:"has-wiki"`
	AllowSquashMerge         bool `yaml:"allow-squash-merge"`
	AllowMergeCommit         bool `yaml:"allow-merge-commit"`
	AllowRebaseMerge         bool `yaml:"allow-rebase-merge"`
	AllowAutoMerge           bool `yaml:"allow-auto-merge"`
	DeleteBranchOnMerge      bool `yaml:"delete-branch-on-merge"`
	AllowForking             bool `yaml:"allow-forking"`
	WebCommitSignoffRequired bool `yaml:"web-commit-signoff-required"`
}

type Dependabot struct {
	Enabled bool `yaml:"enabled"`
}

type DependabotFile struct {
	Enabled bool `yaml:"enabled"`
}

type License struct {
	Enabled bool     `yaml:"enabled"`
	Allowed []string `yaml:"allowed"` // SPDX ids; empty = any license passes
}

type Rulesets struct {
	Enabled bool         `yaml:"enabled"`
	Rules   RulesetRules `yaml:"rules"`
}

type MergeType string

const (
	MergeMethod  MergeType = "merge"
	SquashMethod MergeType = "squash"
	RebaseMethod MergeType = "rebase"
)

type RulesetRules struct {
	BlockForcePush       bool `yaml:"block-force-push"`
	BlockDeletion        bool `yaml:"block-deletion"`
	RequireSignatures    bool `yaml:"require-signatures"`
	RequireLinearHistory bool `yaml:"require-linear-history"`

	// Pull-request rule options; only enforced when require-pr is true.
	RequirePR               bool        `yaml:"require-pr"`
	RequiredApprovals       int         `yaml:"required-approvals"`
	DismissStaleReviews     bool        `yaml:"dismiss-stale-reviews"`
	RequireCodeOwnerReview  bool        `yaml:"require-code-owner-review"`
	RequireLastPushApproval bool        `yaml:"require-last-push-approval"`
	RequireThreadResolution bool        `yaml:"require-thread-resolution"`
	AllowedMergeMethods     []MergeType `yaml:"allowed-merge-methods"` // empty = any

	// Status-check rule options; enforced when required-status-checks is non-empty.
	RequiredStatusChecks []string `yaml:"required-status-checks"`
	StrictStatusChecks   bool     `yaml:"strict-status-checks"`
}

func Defaults() Policy {
	return Policy{Checks: Checks{
		SecretScanning: SecretScanning{Enabled: true, PushProtection: true},
		CodeQL:         CodeQL{Enabled: true},
		Configuration: Configuration{
			Enabled:                  false,
			HasIssues:                true,
			HasProjects:              true,
			HasWiki:                  true,
			AllowSquashMerge:         true,
			AllowMergeCommit:         false,
			AllowRebaseMerge:         true,
			AllowAutoMerge:           false,
			DeleteBranchOnMerge:      false,
			AllowForking:             false,
			WebCommitSignoffRequired: false,
		},
		Dependabot:     Dependabot{Enabled: true},
		DependabotFile: DependabotFile{Enabled: false},
		License:        License{Enabled: true},
		Rulesets: Rulesets{Enabled: true, Rules: RulesetRules{
			BlockForcePush: true,
			BlockDeletion:  true,
		}},
	}}
}

// Parse reads YAML over the defaults: keys present override, absent keep defaults.
// Unknown keys are an error.
func Parse(r io.Reader) (Policy, error) {
	p := Defaults()
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return Policy{}, fmt.Errorf("parsing policy: %w", err)
	}
	return p, nil
}
