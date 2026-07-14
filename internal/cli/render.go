// Package cli renders results and drives interaction.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/GustavoCaso/gh-repocheck/internal/check"
)

func symbol(r check.Result) string {
	if r.Error != nil {
		return "!"
	}
	switch r.Status {
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
func RenderHuman(w io.Writer, results []check.Result) {
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
		if r.Error != nil {
			msg = "error: " + r.Error.Error()
		} else if len(r.Findings) > 0 {
			msg = r.Findings[0].Message
			var msgSb47 strings.Builder
			for _, f := range r.Findings[1:] {
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

func RenderJSON(w io.Writer, results []check.Result) error {
	rows := make([]jsonRow, 0, len(results))
	for _, r := range results {
		row := jsonRow{
			Repo:  r.Repo.FullName(),
			Check: r.Check.ID(),
		}
		if r.Error != nil {
			row.Status = "error"
			row.Message = r.Error.Error()
		} else {
			row.Status = r.Status.String()
			for i, f := range r.Findings {
				if i > 0 {
					row.Message += "; "
				}
				row.Message += f.Message
			}
			_, fixable := r.Check.(check.Fixable)
			row.Fixable = fixable && r.Status == check.Fail
		}
		rows = append(rows, row)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// HasFailures reports whether any result is a failure or error (for exit code).
func HasFailures(results []check.Result) bool {
	for _, r := range results {
		if r.Error != nil || r.Status == check.Fail {
			return true
		}
	}
	return false
}
