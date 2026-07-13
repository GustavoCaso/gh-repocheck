package githubapi

import (
	"context"
	"fmt"
)

// Repo is the target repository a check runs against.
type Repo struct {
	Owner         string
	Name          string
	DefaultBranch string
	Private       bool
	Archived      bool
	Fork          bool
}

func (r Repo) FullName() string { return r.Owner + "/" + r.Name }

// apiRepo maps the GitHub REST repository payload to Repo.
type apiRepo struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
	Fork          bool   `json:"fork"`
}

func (a apiRepo) toRepo() Repo {
	return Repo{
		Owner:         a.Owner.Login,
		Name:          a.Name,
		DefaultBranch: a.DefaultBranch,
		Private:       a.Private,
		Archived:      a.Archived,
		Fork:          a.Fork,
	}
}

// FetchRepo fetches a single repository by owner and name.
func FetchRepo(ctx context.Context, client Client, owner, name string) (Repo, error) {
	var a apiRepo
	if err := client.Get(ctx, fmt.Sprintf("repos/%s/%s", owner, name), &a); err != nil {
		return Repo{}, err
	}
	return a.toRepo(), nil
}

// ListOwnerRepos lists a user's or org's repositories, filtered per flags.
func ListOwnerRepos(ctx context.Context, client Client, owner string, includeArchived, includeForks bool) ([]Repo, error) {
	return listRepos(ctx, client, includeArchived, includeForks, func(page int) string {
		return fmt.Sprintf("users/%s/repos?per_page=100&page=%d", owner, page)
	})
}

// ListOrgRepos lists an organization's repositories (public and private, per
// token access), filtered per flags. Returns a 404 error for non-org owners.
func ListOrgRepos(ctx context.Context, client Client, org string, includeArchived, includeForks bool) ([]Repo, error) {
	return listRepos(ctx, client, includeArchived, includeForks, func(page int) string {
		return fmt.Sprintf("orgs/%s/repos?per_page=100&page=%d", org, page)
	})
}

// ListViewerRepos lists the authenticated user's own repositories, filtered per flags.
func ListViewerRepos(ctx context.Context, client Client, includeArchived, includeForks bool) ([]Repo, error) {
	return listRepos(ctx, client, includeArchived, includeForks, func(page int) string {
		return fmt.Sprintf("user/repos?affiliation=owner&per_page=100&page=%d", page)
	})
}

// listRepos pages through a repo list endpoint until an empty batch.
func listRepos(ctx context.Context, client Client, includeArchived, includeForks bool, path func(page int) string) ([]Repo, error) {
	var out []Repo
	for page := 1; ; page++ {
		var batch []apiRepo
		if err := client.Get(ctx, path(page), &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return out, nil
		}
		for _, a := range batch {
			if a.Archived && !includeArchived {
				continue
			}
			if a.Fork && !includeForks {
				continue
			}
			out = append(out, a.toRepo())
		}
	}
}
