package githubapi

import (
	"context"
	"testing"
)

func TestFetchRepo(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{
		"GET repos/o/r": {Body: `{
			"name":"r","owner":{"login":"o"},"default_branch":"main",
			"private":true,"archived":false,"fork":false}`},
	}}
	repo, err := FetchRepo(context.Background(), stub, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	want := Repo{Owner: "o", Name: "r", DefaultBranch: "main", Private: true}
	if repo != want {
		t.Errorf("repo = %+v, want %+v", repo, want)
	}
}

func TestListOwnerReposFiltersArchivedAndForks(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{
		"GET users/o/repos?per_page=100&page=1": {Body: `[
			{"name":"a","owner":{"login":"o"},"default_branch":"main"},
			{"name":"b","owner":{"login":"o"},"default_branch":"main","archived":true},
			{"name":"c","owner":{"login":"o"},"default_branch":"main","fork":true}]`},
		"GET users/o/repos?per_page=100&page=2": {Body: `[]`},
	}}

	repos, err := ListOwnerRepos(context.Background(), stub, "o", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "a" {
		t.Errorf("default filtering: repos = %+v, want just a", repos)
	}

	repos, err = ListOwnerRepos(context.Background(), stub, "o", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Errorf("includeArchived+includeForks: got %d repos, want 3", len(repos))
	}
}

func TestListOrgRepos(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{
		"GET orgs/acme/repos?per_page=100&page=1": {Body: `[
			{"name":"a","owner":{"login":"acme"},"default_branch":"main"},
			{"name":"b","owner":{"login":"acme"},"default_branch":"main","archived":true},
			{"name":"c","owner":{"login":"acme"},"default_branch":"main","fork":true}]`},
		"GET orgs/acme/repos?per_page=100&page=2": {Body: `[]`},
	}}

	repos, err := ListOrgRepos(context.Background(), stub, "acme", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].FullName() != "acme/a" {
		t.Errorf("default filtering: repos = %+v, want just acme/a", repos)
	}

	repos, err = ListOrgRepos(context.Background(), stub, "acme", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Errorf("includeArchived+includeForks: got %d repos, want 3", len(repos))
	}
}

func TestListOrgReposNotAnOrg(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{}}
	_, err := ListOrgRepos(context.Background(), stub, "someone", false, false)
	if StatusCode(err) != 404 {
		t.Errorf("err = %v, want 404", err)
	}
}

func TestListViewerRepos(t *testing.T) {
	stub := &Stub{Responses: map[string]StubResponse{
		"GET user/repos?affiliation=owner&per_page=100&page=1": {Body: `[
			{"name":"mine","owner":{"login":"me"},"default_branch":"main"}]`},
		"GET user/repos?affiliation=owner&per_page=100&page=2": {Body: `[]`},
	}}
	repos, err := ListViewerRepos(context.Background(), stub, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].FullName() != "me/mine" {
		t.Errorf("repos = %+v", repos)
	}
}
