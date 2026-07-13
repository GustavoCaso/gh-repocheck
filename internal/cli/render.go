// Package cli renders results and drives interaction.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
	"github.com/GustavoCaso/gh-repocheck/internal/runner"
)

func symbol(r runner.CheckResult) string {
	if r.Err != nil {
		return "!"
	}
	switch r.Result.Status {
	case check.Unknown:
		return "?"
	case check.Pass:
		return "✓"
	case check.Fail:
		return "✗"
	case check.Warn:
		return "⚠"
	case check.Skip:
		return "-"
	}
	return "?"
}

// RenderHuman prints results grouped by repo. Results must be grouped already
// (consecutive entries share a repo).
func RenderHuman(w io.Writer, results []runner.CheckResult) {
	lastRepo := ""
	for _, r := range results {
		if full := r.Repo.FullName(); full != lastRepo {
			if lastRepo != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, full)
			lastRepo = full
		}
		msg := ""
		if r.Err != nil {
			msg = "error: " + r.Err.Error()
		} else if len(r.Result.Findings) > 0 {
			msg = r.Result.Findings[0].Message
			var msgSb47 strings.Builder
			for _, f := range r.Result.Findings[1:] {
				msgSb47.WriteString("; " + f.Message)
			}
			msg += msgSb47.String()
		}
		fmt.Fprintf(w, "  %s %-18s %s\n", symbol(r), r.Check.ID(), msg)
	}
}

type jsonRow struct {
	Repo    string `json:"repo"`
	Check   string `json:"check"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Fixable bool   `json:"fixable"`
}

func RenderJSON(w io.Writer, results []runner.CheckResult) error {
	rows := make([]jsonRow, 0, len(results))
	for _, r := range results {
		row := jsonRow{
			Repo:  r.Repo.FullName(),
			Check: r.Check.ID(),
		}
		if r.Err != nil {
			row.Status = "error"
			row.Message = r.Err.Error()
		} else {
			row.Status = r.Result.Status.String()
			for i, f := range r.Result.Findings {
				if i > 0 {
					row.Message += "; "
				}
				row.Message += f.Message
			}
			_, fixable := r.Check.(check.Fixable)
			row.Fixable = fixable && r.Result.Status == check.Fail
		}
		rows = append(rows, row)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// HasFailures reports whether any result is a failure or error (for exit code).
func HasFailures(results []runner.CheckResult) bool {
	for _, r := range results {
		if r.Err != nil || r.Result.Status == check.Fail {
			return true
		}
	}
	return false
}
