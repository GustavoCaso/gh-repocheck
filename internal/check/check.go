// Package check defines the core types every repocheck check implements.
package check

import (
	"context"

	"github.com/GustavoCaso/gh-repocheck/internal/githubapi"
	"github.com/GustavoCaso/gh-repocheck/internal/policy"
)

type Status int

const (
	Unknown Status = iota // zero value: not a valid check outcome
	Pass
	Fail
	Warn
	Skip
)

const unknownStatusName = "unknown"

func (s Status) String() string {
	switch s {
	case Unknown:
		return unknownStatusName
	case Pass:
		return "pass"
	case Fail:
		return "fail"
	case Warn:
		return "warn"
	case Skip:
		return "skip"
	}
	return unknownStatusName
}

// Repo is the target repository a check runs against.
type Repo = githubapi.Repo

type Finding struct {
	Message string
	FixHint string // human description of what Fix would do; empty if not fixable
}

type Result struct {
	Status   Status
	Findings []Finding
}

type Check interface {
	ID() string
	Description() string
	Run(ctx context.Context, client githubapi.Client, repo Repo, pol policy.Policy) (Result, error)
}

// Fixable checks can remediate their own failures.
type Fixable interface {
	Check
	Fix(ctx context.Context, client githubapi.Client, repo Repo, pol policy.Policy) error
}
