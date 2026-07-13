// Package checks contains the built-in repocheck checks.
package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type featureStatus struct {
	Status string `json:"status"`
}

type securityAndAnalysis struct {
	SecretScanning               *featureStatus `json:"secret_scanning,omitempty"`
	SecretScanningPushProtection *featureStatus `json:"secret_scanning_push_protection,omitempty"`
}

type SecretScanning struct{}

var _ check.Fixable = (*SecretScanning)(nil)

func (s *SecretScanning) ID() string { return "secret-scanning" }
func (s *SecretScanning) Description() string {
	return "Secret scanning and push protection are enabled"
}

func (s *SecretScanning) Run(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) (check.Result, error) {
	var resp struct {
		SecurityAndAnalysis *securityAndAnalysis `json:"security_and_analysis"`
	}
	if err := client.Get(ctx, fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name), &resp); err != nil {
		return check.Result{}, err
	}
	sa := resp.SecurityAndAnalysis
	if sa == nil || sa.SecretScanning == nil {
		if repo.Private {
			return check.Result{Status: check.Skip, Findings: []check.Finding{{
				Message: "secret scanning unavailable (private repo without GitHub Advanced Security)",
			}}}, nil
		}
		return check.Result{Status: check.Skip, Findings: []check.Finding{{
			Message: "secret scanning status not reported for this repo",
		}}}, nil
	}
	var findings []check.Finding
	if sa.SecretScanning.Status != "enabled" {
		findings = append(findings, check.Finding{
			Message: "secret scanning is disabled",
			FixHint: "enable secret scanning",
		})
	}
	if pol.Checks.SecretScanning.PushProtection &&
		(sa.SecretScanningPushProtection == nil || sa.SecretScanningPushProtection.Status != "enabled") {
		findings = append(findings, check.Finding{
			Message: "push protection is disabled",
			FixHint: "enable push protection",
		})
	}
	if len(findings) > 0 {
		return check.Result{Status: check.Fail, Findings: findings}, nil
	}
	return check.Result{Status: check.Pass}, nil
}

func (s *SecretScanning) Fix(ctx context.Context, client githubapi.Client, repo check.Repo, pol policy.Policy) error {
	sa := securityAndAnalysis{
		SecretScanning: &featureStatus{Status: "enabled"},
	}
	if pol.Checks.SecretScanning.PushProtection {
		sa.SecretScanningPushProtection = &featureStatus{Status: "enabled"}
	}
	body, err := json.Marshal(map[string]any{"security_and_analysis": sa})
	if err != nil {
		return err
	}
	return client.Patch(ctx, fmt.Sprintf("repos/%s/%s", repo.Owner, repo.Name), bytes.NewReader(body), nil)
}
