package githubapi

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestFetchRulesetsFetchesDetails(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Body: `[
			{"id":1,"name":"protect","target":"branch","enforcement":"active"},
			{"id":2,"name":"release","target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/main"]}},
			"rules":[{"type":"deletion"}]}`},
		"GET repos/o/r/rulesets/2": {Body: `{
			"id":2,"name":"release","target":"branch","enforcement":"active",
			"conditions":{"ref_name":{"include":["refs/heads/release"]}},
			"rules":[{"type":"non_fast_forward"}]}`},
	}}
	got, err := FetchRulesets(context.Background(), stub, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Name != "protect" || len(got[0].Rules) != 1 || got[0].Rules[0].Type != "deletion" {
		t.Errorf("first ruleset = %+v", got[0])
	}
	if got[1].Name != "release" || got[1].Conditions.RefName.Include[0] != "refs/heads/release" {
		t.Errorf("second ruleset = %+v", got[1])
	}
}

func TestFetchRulesetsOnlyFetchesRequestedNames(t *testing.T) {
	// Only ruleset 1's detail is stubbed; fetching ruleset 2 would come back
	// 404 and appear as an uninspectable entry. Filtering by name must skip
	// it entirely.
	stub := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Body: `[
			{"id":1,"name":"protect","target":"branch","enforcement":"active"},
			{"id":2,"name":"unrelated","target":"branch","enforcement":"active"}]`},
		"GET repos/o/r/rulesets/1": {Body: `{
			"id":1,"name":"protect","target":"branch","enforcement":"active",
			"rules":[{"type":"deletion"}]}`},
	}}
	got, err := FetchRulesets(context.Background(), stub, "o", "r", "protect")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "protect" || got[0].Uninspectable {
		t.Fatalf("rulesets = %+v, want only inspectable %q", got, "protect")
	}
}

func TestFetchRulesetsMarksUninspectableOn404(t *testing.T) {
	// Org-inherited rulesets 404 on the repo-level detail endpoint
	// (unstubbed here); the summary must survive with Uninspectable set.
	stub := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":7,"name":"org-rules","target":"branch","enforcement":"active"}]`,
		},
	}}
	got, err := FetchRulesets(context.Background(), stub, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Uninspectable || got[0].Name != "org-rules" || got[0].ID != 7 {
		t.Fatalf("rulesets = %+v, want one uninspectable summary", got)
	}
}

func TestFetchRulesetsPaginates(t *testing.T) {
	// Full first page forces a second request; short second page stops.
	var page1 []string
	responses := map[string]StubResponse{}
	for i := 1; i <= 100; i++ {
		page1 = append(page1, fmt.Sprintf(`{"id":%d,"name":"rs%d","target":"branch","enforcement":"active"}`, i, i))
		responses[fmt.Sprintf("GET repos/o/r/rulesets/%d", i)] = StubResponse{
			Body: fmt.Sprintf(`{"id":%d,"name":"rs%d"}`, i, i),
		}
	}
	responses["GET repos/o/r/rulesets?per_page=100&page=1"] = StubResponse{Body: "[" + strings.Join(page1, ",") + "]"}
	responses["GET repos/o/r/rulesets?per_page=100&page=2"] = StubResponse{
		Body: `[{"id":101,"name":"rs101","target":"branch","enforcement":"active"}]`,
	}
	responses["GET repos/o/r/rulesets/101"] = StubResponse{Body: `{"id":101,"name":"rs101"}`}

	got, err := FetchRulesets(context.Background(), &Stub{Responses: responses}, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 101 || got[100].Name != "rs101" {
		t.Fatalf("len = %d, want 101 across two pages", len(got))
	}
}

func TestFetchRulesetsPropagatesErrors(t *testing.T) {
	listErr := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {Status: 500},
	}}
	if _, err := FetchRulesets(context.Background(), listErr, "o", "r"); err == nil {
		t.Error("list error not propagated")
	}
	detailErr := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r/rulesets?per_page=100&page=1": {
			Body: `[{"id":1,"name":"protect","target":"branch","enforcement":"active"}]`,
		},
		"GET repos/o/r/rulesets/1": {Status: 500},
	}}
	if _, err := FetchRulesets(context.Background(), detailErr, "o", "r"); err == nil {
		t.Error("non-404 detail error not propagated")
	}
}
