package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type RulesetSummary struct {
	ID          int    `json:"id"`
	Target      string `json:"target"`
	Name        string `json:"name"`
	Enforcement string `json:"enforcement"`
}

type BypassActor struct {
	ActorID    int    `json:"actor_id"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

type RulesetRule struct {
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
}

type Ruleset struct {
	RulesetSummary

	BypassActors []BypassActor `json:"bypass_actors"`
	Conditions   struct {
		RefName struct {
			Include []string `json:"include"`
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []RulesetRule `json:"rules"`

	// Uninspectable marks a ruleset whose detail endpoint 404ed at the repo
	// level (org-inherited); only the summary fields are populated.
	Uninspectable bool `json:"-"`
}

func FetchRuleset(ctx context.Context, client Client, owner, name string, id int) (Ruleset, error) {
	var ruleset Ruleset
	if err := client.Get(ctx, fmt.Sprintf("repos/%s/%s/rulesets/%d", owner, name, id), &ruleset); err != nil {
		return Ruleset{}, err
	}
	return ruleset, nil
}

// FetchRulesets pages through a repo's rulesets and fetches each one's
// detail. When only is non-empty, rulesets with other names are skipped
// without fetching their detail, keeping the request count at one per page
// plus one per wanted ruleset. Org-inherited rulesets appear in the list but
// their detail endpoint 404s at the repo level; those come back with only
// summary fields and Uninspectable set.
func FetchRulesets(ctx context.Context, client Client, owner, name string, only ...string) ([]Ruleset, error) {
	const perPage = 100
	wanted := map[string]bool{}
	for _, n := range only {
		wanted[n] = true
	}
	var out []Ruleset
	for page := 1; ; page++ {
		var summaries []RulesetSummary
		path := fmt.Sprintf("repos/%s/%s/rulesets?per_page=%d&page=%d", owner, name, perPage, page)
		if err := client.Get(ctx, path, &summaries); err != nil {
			return nil, err
		}
		for _, summary := range summaries {
			if len(wanted) > 0 && !wanted[summary.Name] {
				continue
			}
			ruleset, err := FetchRuleset(ctx, client, owner, name, summary.ID)
			if err != nil {
				if StatusCode(err) == http.StatusNotFound {
					out = append(out, Ruleset{RulesetSummary: summary, Uninspectable: true})
					continue
				}
				return nil, err
			}
			out = append(out, ruleset)
		}
		if len(summaries) < perPage {
			return out, nil
		}
	}
}
