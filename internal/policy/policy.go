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
	SecretScanning SecretScanning `yaml:"secret_scanning"`
	CodeQL         CodeQL         `yaml:"codeql"`
	Configuration  Configuration  `yaml:"configuration"`
	Dependabot     Dependabot     `yaml:"dependabot"`
	DependabotFile DependabotFile `yaml:"dependabot_file"`
	License        License        `yaml:"license"`
	Rulesets       Rulesets       `yaml:"rulesets"`
}

type SecretScanning struct {
	Enabled        bool `yaml:"enabled"`
	PushProtection bool `yaml:"push_protection"`
}

type CodeQL struct {
	Enabled bool `yaml:"enabled"`
}

type Configuration struct {
	Enabled                  bool `yaml:"enabled"`
	HasIssues                bool `yaml:"has_issues"`
	HasProjects              bool `yaml:"has_projects"`
	HasWiki                  bool `yaml:"has_wiki"`
	AllowSquashMerge         bool `yaml:"allow_squash_merge"`
	AllowMergeCommit         bool `yaml:"allow_merge_commit"`
	AllowRebaseMerge         bool `yaml:"allow_rebase_merge"`
	AllowAutoMerge           bool `yaml:"allow_auto_merge"`
	DeleteBranchOnMerge      bool `yaml:"delete_branch_on_merge"`
	WebCommitSignoffRequired bool `yaml:"web_commit_signoff_required"`
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
	Enabled bool                      `yaml:"enabled"`
	Rules   map[string][]RulesetRules `yaml:"rules"`
}

type MergeType string

const (
	MergeMethod  MergeType = "merge"
	SquashMethod MergeType = "squash"
	RebaseMethod MergeType = "rebase"
)

type BypassMode string

const (
	AlwaysMode      BypassMode = "always"
	PullRequestMode BypassMode = "pull_request"
	ExemptMode      BypassMode = "exempt"
)

type RulesetRules struct {
	Name                 string `yaml:"name"`
	BlockForcePush       bool   `yaml:"block_force_push"`
	BlockDeletion        bool   `yaml:"block_deletion"`
	RequireSignatures    bool   `yaml:"require_signatures"`
	RequireLinearHistory bool   `yaml:"require_linear_history"`

	// Bypass
	BypassByAdminRole      bool       `yaml:"bypass_by_admin_role"`
	BypassModeAdmin        BypassMode `yaml:"bypass_mode_admin"`
	BypassByMaintainerRole bool       `yaml:"bypass_by_maintainer_role"`
	BypassModeMaintainer   BypassMode `yaml:"bypass_mode_maintainer"`
	BypassByWriterRole     bool       `yaml:"bypass_by_writer_role"`
	BypassModeWriter       BypassMode `yaml:"bypass_mode_writer"`

	// Pull request rule options; only enforced when require_pr is true.
	RequirePR               bool        `yaml:"require_pr"`
	RequiredApprovals       int         `yaml:"required_approvals"`
	DismissStaleReviews     bool        `yaml:"dismiss_stale_reviews"`
	RequireCodeOwnerReview  bool        `yaml:"require_code_owner_review"`
	RequireLastPushApproval bool        `yaml:"require_last_push_approval"`
	RequireThreadResolution bool        `yaml:"require_thread_resolution"`
	AllowedMergeMethods     []MergeType `yaml:"allowed_merge_methods"` // empty = any

	// Status check rule options; enforced when required_status_checks is non_empty.
	RequiredStatusChecks []string `yaml:"required_status_checks"`
	StrictStatusChecks   bool     `yaml:"strict_status_checks"`
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
			WebCommitSignoffRequired: false,
		},
		Dependabot:     Dependabot{Enabled: true},
		DependabotFile: DependabotFile{Enabled: false},
		License:        License{Enabled: true},
		Rulesets:       Rulesets{Enabled: false},
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
