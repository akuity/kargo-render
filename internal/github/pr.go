package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"

	"github.com/akuity/kargo-render/pkg/git"
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
	var baseURL string
	isEnterprise, baseURL, owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return "", err
	}
	var githubClient *github.Client
	if isEnterprise {
		githubClient, err = github.NewEnterpriseClient(
			baseURL,
			baseURL,
			oauth2.NewClient(
				ctx,
				oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: repoCreds.Password},
				),
			),
		)
		if err != nil {
			return "", err
		}
	} else {
		githubClient = github.NewClient(
			oauth2.NewClient(
				ctx,
				oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: repoCreds.Password},
				),
			),
		)
	}
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
			fmt.Errorf("error opening pull request to the target branch: %w", err)
	}
	return *pr.HTMLURL, nil
}

func parseGitHubURL(url string) (bool, string, string, string, error) {
	// regex := regexp.MustCompile(`^https\://github\.[\w]/([\w-]+)/([\w-]+).*`)
	regex := regexp.MustCompile(`^https\://([\w.-]+)/([\w-]+)/([\w-]+).*`)
	parts := regex.FindStringSubmatch(url)
	if len(parts) != 4 {
		return false, "", "", "", fmt.Errorf("error parsing github repository URL %q", url)
	}
	isEnterprise := false
	if parts[1] != "github.com" {
		isEnterprise = true
	}
	baseURL := "https://" + parts[1]

	return isEnterprise, baseURL, parts[2], parts[3], nil
}
