package github

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/go-github/v47/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/akuityio/bookkeeper/internal/git"
)

func OpenPR(
	ctx context.Context,
	repoURL string,
	title string,
	body string,
	targetBranch string,
	commitBranch string,
	repoCreds git.RepoCredentials,
) (string, error) {
	owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return "", err
	}
	githubClient := github.NewClient(
		oauth2.NewClient(
			ctx,
			oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: repoCreds.Password},
			),
		),
	)
	pr, _, err := githubClient.PullRequests.Create(
		ctx,
		owner,
		repo,
		&github.NewPullRequest{
			Title:               github.String(title),
			Base:                github.String(targetBranch),
			Head:                github.String(commitBranch),
			Body:                github.String(body),
			MaintainerCanModify: github.Bool(false),
		},
	)
	if err != nil {
		// If the error is simply that a PR already exists for this branch, that's
		// fine. Just ignore that.
		if strings.Contains(err.Error(), "A pull request already exists for") {
			return "", nil
		}
		return "",
			errors.Wrap(err, "error opening pull request to the target branch")
	}
	return *pr.HTMLURL, nil
}

func parseGitHubURL(url string) (string, string, error) {
	regex := regexp.MustCompile(`^https\://github\.com/([\w-]+)/([\w-]+).*`)
	parts := regex.FindStringSubmatch(url)
	if len(parts) != 3 {
		return "", "", errors.Errorf("error parsing github repository URL %q", url)
	}
	return parts[1], parts[2], nil
}
