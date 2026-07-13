package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Resolve picks the effective policy. Order: explicit flag path,
// repo .github/repocheck.yml content (fetched by the caller, nil if absent),
// user config path, built-in defaults. Returns the policy and its source label.
func Resolve(flagPath string, repoContent []byte, userPath string) (Policy, string, error) {
	if flagPath != "" {
		p, err := parseFile(flagPath)
		return p, flagPath, err
	}
	if repoContent != nil {
		p, err := Parse(bytes.NewReader(repoContent))
		if err != nil {
			return Policy{}, "", fmt.Errorf(".github/repocheck.yml: %w", err)
		}
		return p, ".github/repocheck.yml", nil
	}
	f, err := os.Open(userPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Defaults(), "defaults", nil
		}
		return Policy{}, "", err
	}
	defer f.Close()
	p, err := Parse(f)
	if err != nil {
		return Policy{}, "", fmt.Errorf("%s: %w", userPath, err)
	}
	return p, userPath, nil
}

func parseFile(path string) (Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return Policy{}, err
	}
	defer f.Close()
	p, err := Parse(f)
	if err != nil {
		return Policy{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// UserConfigPath is where the per-user policy lives.
func UserConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gh-repocheck", "policy.yml")
}
